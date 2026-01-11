package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os/exec"
	"strings"

	"github.com/erh/vmodutils"

	"go.viam.com/rdk/logging"
)

type OnehelmAppConfig struct {
	Port     int
	AppName  string
	AppId    string
	IconPath string
}

func (c *OnehelmAppConfig) Valid() error {
	if c.Port == 0 {
		c.Port = 8080
	}
	if c.AppName == "" {
		c.AppName = "TestOnehelmApp"
	}
	if c.AppId == "" {
		return fmt.Errorf("nede an AppId")
	}
	if c.IconPath == "" {
		return fmt.Errorf("need an IconPath")
	}
	return nil
}

type Close func(ctx context.Context)

type Start func(ctx context.Context) Close

func PrepOnehelmServer(fs fs.FS, logger logging.Logger, cfg *OnehelmAppConfig) (*http.ServeMux, Start, error) {
	err := cfg.Valid()
	if err != nil {
		return nil, nil, err
	}

	mux, server, err := vmodutils.PrepInModuleServer(fs, logger.Sublogger("accesslog"), nil)
	if err != nil {
		return nil, nil, err
	}
	server.Addr = fmt.Sprintf(":%d", cfg.Port)

	mux.HandleFunc("/garmin.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header()["Content-Type"] = []string{"application/json"}
		m := map[string]string{
			"id":    cfg.AppId,
			"path":  "/",
			"title": cfg.AppName,
			"icon":  cfg.IconPath,
		}
		data, err := json.Marshal(m)
		if err != nil {
			panic(err)
		}
		w.Write(data)
	})

	if strings.HasPrefix(cfg.IconPath, "http") {
		imgData, mimeType, err := DownloadBytes(cfg.IconPath)
		if err != nil {
			return nil, nil, fmt.Errorf("can't download icon %w from: %s", err, cfg.IconPath)
		}
		cfg.IconPath = "/magicIcon"

		mux.HandleFunc(cfg.IconPath, func(w http.ResponseWriter, r *http.Request) {
			w.Header()["Content-Type"] = []string{mimeType}
			w.Write(imgData)
		})

	}

	return mux, func(ctx context.Context) Close {
		ctx, cancel := context.WithCancel(ctx)

		avahi, err := startAvahiDaemon(ctx, cfg, logger)
		if err != nil {
			logger.Warnf("can't start avahi daemon: %v", err)
		} else {
			err = avahi.Start()
			if err != nil {
				logger.Warnf("can't start avahi daemon: %v", err)
				avahi = nil
			}
		}

		go func() {
			logger.Infof("starting webserver for onehelm on %v", server.Addr)
			err := server.ListenAndServe()
			if err != nil {
				logger.Errorf("ListenAndServe error: %v", err)
				server = nil
			}
		}()

		return func(ctx context.Context) {
			cancel()
			if avahi != nil {
				avahi.Wait()
				avahi = nil
			}
			if server != nil {
				server.Close()
				server = nil
			}
		}
	}, nil

}

func startAvahiDaemon(ctx context.Context, cfg *OnehelmAppConfig, logger logging.Logger) (*exec.Cmd, error) {
	garminNetwork, err := FindGarminInterface()
	if err != nil {
		return nil, err
	}
	if !garminNetwork.Good() {
		return nil, fmt.Errorf("no garmin network found")
	}

	logger.Infof("registering %v for one helm app", cfg.AppName)

	return exec.CommandContext(ctx, "/usr/bin/avahi-publish-service",
		cfg.AppName,
		"_garmin-mrn-html._tcp",
		fmt.Sprintf("%d", cfg.Port),
		"protovers=1",
		"path=/garmin.json",
	), nil
}
