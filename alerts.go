package verhboat

import (
	"context"
	"fmt"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var AlertsSensorModel = NamespaceFamily.WithModel("alerts")

func init() {
	resource.RegisterComponent(
		sensor.API,
		AlertsSensorModel,
		resource.Registration[sensor.Sensor, *AlertsSensorConfig]{
			Constructor: newAlertsSensor,
		})
}

type AlertsSensorConfig struct {
	FreshwaterTank     string  `json:"freshwater_tank"`
	FreshwaterSpotZero string  `json:"freshwater_spotzero"`
	AlertLevel         float64 `json:"alert_level"`
}

func (c *AlertsSensorConfig) Validate(_ string) ([]string, []string, error) {
	if c.FreshwaterTank == "" {
		return nil, nil, fmt.Errorf("need freshwater_tank")
	}

	if c.FreshwaterSpotZero == "" {
		return nil, nil, fmt.Errorf("need freshwater_spotzero")
	}

	return []string{c.FreshwaterTank, c.FreshwaterSpotZero}, nil, nil
}

func (c *AlertsSensorConfig) alertLevel() float64 {
	if c.AlertLevel <= 0 {
		return 99
	}
	return c.AlertLevel
}

func newAlertsSensor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (sensor.Sensor, error) {
	conf, err := resource.NativeConfig[*AlertsSensorConfig](rawConf)
	if err != nil {
		return nil, err
	}

	return NewAlertsSensor(ctx, deps, rawConf.ResourceName(), conf, logger)
}

func NewAlertsSensor(ctx context.Context, deps resource.Dependencies, name resource.Name, conf *AlertsSensorConfig, logger logging.Logger) (*AlertsSensorData, error) {
	var err error

	d := &AlertsSensorData{
		name:   name,
		logger: logger,
		conf:   conf,
	}

	d.fwTank, err = sensor.FromDependencies(deps, conf.FreshwaterTank)
	if err != nil {
		return nil, err
	}

	d.fwSpotZero, err = sensor.FromDependencies(deps, conf.FreshwaterSpotZero)
	if err != nil {
		return nil, err
	}

	return d, nil
}

type AlertsSensorData struct {
	resource.AlwaysRebuild

	name   resource.Name
	conf   *AlertsSensorConfig
	logger logging.Logger

	fwTank     sensor.Sensor
	fwSpotZero sensor.Sensor
}

func (asd *AlertsSensorData) getData(ctx context.Context) (map[string]interface{}, error) {
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

	flow, ok := sz["Product Water Flow"].(float64)
	if !ok {
		return nil, fmt.Errorf("spotzero data has no flow %v", sz)
	}

	asd.logger.Infof("level %0.2f flow: %0.2f", level, flow)

	m := map[string]interface{}{}
	m["level"] = level
	m["flow"] = flow
	if level >= asd.conf.alertLevel() && flow > 0 {
		m["fwerror"] = fmt.Sprintf("level %0.2f flow: %0.2f", level, flow)
	} else {
		m["fwerror"] = ""
	}

	return m, nil
}

func (asd *AlertsSensorData) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return asd.getData(ctx)
}

func (asd *AlertsSensorData) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (asd *AlertsSensorData) Close(ctx context.Context) error {
	return nil
}

func (asd *AlertsSensorData) Name() resource.Name {
	return asd.name
}
