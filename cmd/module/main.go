package main

import (
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"verhboat"
)

func main() {
	module.ModularMain(
		resource.APIModel{sensor.API, verhboat.AlertsSensorModel},
		resource.APIModel{sensor.API, verhboat.FWFillSensorModel},
		resource.APIModel{toggleswitch.API, verhboat.TahomaHackModel},
	)
}
