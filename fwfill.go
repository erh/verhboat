package verhboat

import (
	"context"
	"fmt"
	"time"

	"go.viam.com/rdk/components/gripper"
	"go.viam.com/rdk/components/sensor"
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

	return []string{c.FreshwaterTank, c.FreshwaterSpotZero, c.FreshwaterValve}, nil, nil
}

func (c *FWFillSensorConfig) GetStartLevel() float64 {
	if c.StartLevel <= 0 {
		return 93
	}
	return c.StartLevel
}

func (c *FWFillSensorConfig) GetEndLevel() float64 {
	if c.EndLevel <= 0 {
		return 98
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
		name:            name,
		logger:          logger,
		conf:            conf,
		lastRandomClose: time.Now(),
	}

	d.fwTank, err = sensor.FromDependencies(deps, conf.FreshwaterTank)
	if err != nil {
		return nil, err
	}

	d.fwSpotZero, err = sensor.FromDependencies(deps, conf.FreshwaterSpotZero)
	if err != nil {
		return nil, err
	}

	d.fwValve, err = gripper.FromDependencies(deps, conf.FreshwaterValve)
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
	fwValve    gripper.Gripper

	lastRandomClose time.Time
}

func (asd *FWFillSensorData) getData(ctx context.Context) (map[string]interface{}, error) {
	tank, err := asd.fwTank.Readings(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("can't read from tank %w", err)
	}

	sz, err := asd.fwSpotZero.Readings(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("can't read from spot zero: %w", err)
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
		"random":  false,
	}

	if time.Since(asd.lastRandomClose) > (10 * time.Minute) {
		m["random"] = true
		_, err = asd.fwValve.Grab(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("couldn't do random valve close: %w", err)
		}
		time.Sleep(time.Second * 5)
		asd.lastRandomClose = time.Now()
	}

	if szState == "Stopping" {
		m["action"] = "close"
	} else if level < asd.conf.GetStartLevel() {
		m["action"] = "open"
	} else if level > asd.conf.GetEndLevel() {
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
		err = asd.fwValve.Open(ctx, nil)
	} else {
		_, err = asd.fwValve.Grab(ctx, nil)
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
