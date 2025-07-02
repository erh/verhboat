package main

import (
	"context"
	"flag"
	"fmt"
	"time"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"

	"github.com/erh/vmodutils"

	"verhboat"
)

func main() {
	err := realMain()
	if err != nil {
		panic(err)
	}
}

func realMain() error {
	ctx := context.Background()
	logger := logging.NewLogger("cli")

	configFile := flag.String("config", "", "config file")
	host := flag.String("host", "", "host to connect to")
	debug := flag.Bool("debug", false, "debugging on")

	flag.Parse()

	logger.Infof("using config file [%s] and host [%s]", *configFile, *host)

	if *configFile == "" {
		return fmt.Errorf("need a config file")
	}

	cfg := &verhboat.AlertsSensorConfig{}

	err := vmodutils.ReadJSONFromFile(*configFile, cfg)
	if err != nil {
		return err
	}

	_, _, err = cfg.Validate("")
	if err != nil {
		return err
	}

	client, err := vmodutils.ConnectToHostFromCLIToken(ctx, *host, logger)
	if err != nil {
		return err
	}
	defer client.Close(ctx)

	deps, err := vmodutils.MachineToDependencies(client)
	if err != nil {
		return err
	}

	svcLogger := logger.Sublogger("module")
	if *debug {
		svcLogger.SetLevel(logging.DEBUG)
	}

	thing, err := verhboat.NewAlertsSensor(ctx, deps, sensor.Named("foo"), cfg, logger)
	if err != nil {
		return err
	}
	defer thing.Close(ctx)

	time.Sleep(5 * time.Second)

	res, err := thing.Readings(ctx, nil)
	if err != nil {
		return err
	}
	logger.Info(res)

	return nil
}
