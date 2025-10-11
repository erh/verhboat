package main

import (
	"flag"
	"fmt"
	"strings"

	"go.viam.com/rdk/logging"

	"github.com/erh/vmodutils"

	"go.viam.com/rdk/components/switch"

	"verhboat"
)

func main() {
	err := realMain()
	if err != nil {
		panic(err)
	}
}

func realMain() error {
	logger := logging.NewLogger("tahoma")

	configFile := flag.String("config", "", "config file")
	debug := flag.Bool("debug", false, "debugging on")
	action := flag.String("action", "", "")
	label := flag.String("label", "", "")

	flag.Parse()

	if *configFile == "" {
		return fmt.Errorf("need a config file")
	}

	if *debug {
		logger.SetLevel(logging.DEBUG)
	}

	cfg := &verhboat.TahomaConfig{}
	err := vmodutils.ReadJSONFromFile(*configFile, cfg)
	if err != nil {
		return err
	}

	_, _, err = cfg.Validate("")
	if err != nil {
		return err
	}

	client, err := verhboat.NewTahomaClient(cfg, toggleswitch.Named("foo"), logger)
	if err != nil {
		return err
	}

	switch *action {
	case "list":
		devices, err := client.GetDevices()
		if err != nil {
			return err
		}

		for _, device := range devices {
			fmt.Printf("%#v\n", device)
		}
	case "up":
		if *label == "" {
			return fmt.Errorf("need a label")
		}
		return client.LiftShadeByLabel(*label)
	case "lower-and-tilt":
		if *label == "" {
			return fmt.Errorf("need a label")
		}
		return client.LowerAndTiltShadeByLabels(strings.Split(*label, ","))

	default:
		return fmt.Errorf("unknown action [%s]", *action)
	}

	return nil
}
