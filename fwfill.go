package verhboat

import (
	"context"
	"fmt"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var FWFillSensorModel = NamespaceFamily.WithModel("fw-fill")

func init() {
	resource.RegisterComponent(
		sensor.API,
		FWFillSensorModel,
		resource.Registration[sensor.Sensor, *FWFillSensorConfig]{
			Constructor: newFWFillSensor,
		})
}

type FWFillSensorConfig struct {
	FreshwaterTank     string `json:"freshwater_tank"`
	FreshwaterSpotZero string `json:"freshwater_spotzero"`
	FreshwaterValve    string `json:"freshwater_valve"`

	StartLevel float64 `json:"start_level"`
	EndLevel   float64 `json:"end_level"`
}

func (c *FWFillSensorConfig) Validate(_ string) ([]string, []string, error) {
	if c.FreshwaterTank == "" {
		return nil, nil, fmt.Errorf("need freshwater_tank")
	}

	if c.FreshwaterSpotZero == "" {
		return nil, nil, fmt.Errorf("need freshwater_spotzero")
	}

	if c.FreshwaterValve == "" {
		return nil, nil, fmt.Errorf("need freshwater_valve")
	}

	return []string{c.FreshwaterTank, c.FreshwaterValve}, []string{c.FreshwaterSpotZero}, nil
}

func (c *FWFillSensorConfig) GetStartLevel() float64 {
	if c.StartLevel <= 0 {
		return 80
	}
	return c.StartLevel
}

func (c *FWFillSensorConfig) GetEndLevel() float64 {
	if c.EndLevel <= 0 {
		return 96
	}
	return c.EndLevel
}

func newFWFillSensor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (sensor.Sensor, error) {
	conf, err := resource.NativeConfig[*FWFillSensorConfig](rawConf)
	if err != nil {
		return nil, err
	}

	return NewFWFillSensor(ctx, deps, rawConf.ResourceName(), conf, logger)
}

func NewFWFillSensor(ctx context.Context, deps resource.Dependencies, name resource.Name, conf *FWFillSensorConfig, logger logging.Logger) (*FWFillSensorData, error) {
	var err error

	d := &FWFillSensorData{
		name:   name,
		logger: logger,
		conf:   conf,
	}

	d.fwTank, err = sensor.FromDependencies(deps, conf.FreshwaterTank)
	if err != nil {
		return nil, err
	}

	if conf.FreshwaterSpotZero != "" {
		d.fwSpotZero, err = sensor.FromDependencies(deps, conf.FreshwaterSpotZero)
		if err != nil {
			return nil, err
		}
	}

	d.fwValve, err = toggleswitch.FromDependencies(deps, conf.FreshwaterValve)
	if err != nil {
		return nil, err
	}

	return d, nil
}

type FWFillSensorData struct {
	resource.AlwaysRebuild

	name   resource.Name
	conf   *FWFillSensorConfig
	logger logging.Logger

	fwTank     sensor.Sensor
	fwSpotZero sensor.Sensor
	fwValve    toggleswitch.Switch
}

func (asd *FWFillSensorData) getData(ctx context.Context) (map[string]interface{}, error) {
	tank, err := asd.fwTank.Readings(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("can't read from tank %w", err)
	}

	sz := map[string]interface{}{}
	if asd.fwSpotZero != nil {
		sz, err = asd.fwSpotZero.Readings(ctx, nil)
		if err != nil {
			asd.logger.Warnf("can't read from spot zero: %w", err)
		}
	}

	asd.logger.Debugf("tank: %v", tank)
	asd.logger.Debugf("sz: %v", sz)

	level, ok := tank["Level"].(float64)
	if !ok {
		return nil, fmt.Errorf("tank data has no level %v", tank)
	}

	gallons := (level / 100) * tank["Capacity"].(float64) * 0.264172

	szState := sz["Watermaker Operating State"]
	gpm := sz["Product Water Flow"].(float64) * 0.00440287

	m := map[string]interface{}{
		"action":  "none",
		"level":   level,
		"gallons": gallons,
		"gpm":     gpm,
		"szState": szState,
	}

	if szState == "Stopping" {
		m["action"] = "close"
	} else if level < asd.conf.GetStartLevel() {
		m["action"] = "open"
	} else if level >= asd.conf.GetEndLevel() {
		m["action"] = "close"
	}

	return m, nil
}

func (asd *FWFillSensorData) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	d, err := asd.getData(ctx)
	if err != nil {
		return nil, err
	}

	if d["action"] == "open" {
		err = asd.fwValve.SetPosition(ctx, 1, nil)
	} else if d["action"] == "close" {
		err = asd.fwValve.SetPosition(ctx, 0, nil)
	}

	return d, err
}

func (asd *FWFillSensorData) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (asd *FWFillSensorData) Close(ctx context.Context) error {
	return nil
}

func (asd *FWFillSensorData) Name() resource.Name {
	return asd.name
}
