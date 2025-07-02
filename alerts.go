package verhboat

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	goutils "go.viam.com/utils"
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

	d.cancelCtx, d.cancel = context.WithCancel(context.Background())

	d.wg.Add(1)
	go d.run(d.cancelCtx)

	return d, nil
}

type AlertsSensorData struct {
	resource.AlwaysRebuild

	name   resource.Name
	conf   *AlertsSensorConfig
	logger logging.Logger

	fwTank     sensor.Sensor
	fwSpotZero sensor.Sensor

	wg        sync.WaitGroup
	cancelCtx context.Context
	cancel    context.CancelFunc

	mu  sync.Mutex
	res map[string]interface{}
}

func (asd *AlertsSensorData) run(ctx context.Context) {
	defer asd.wg.Done()
	for ctx.Err() == nil {
		start := time.Now()

		err := asd.doLoop(ctx)
		if err != nil {
			asd.logger.Warnf("error doing loop: %v", err)
		}
		goutils.SelectContextOrWait(ctx, time.Minute-time.Since(start))
	}
}

func (asd *AlertsSensorData) doLoop(ctx context.Context) error {
	tank, err := asd.fwTank.Readings(ctx, nil)
	if err != nil {
		return fmt.Errorf("can't read from tank %w", err)
	}

	sz, err := asd.fwSpotZero.Readings(ctx, nil)
	if err != nil {
		return fmt.Errorf("can't read from spot zero: %w", err)
	}

	asd.logger.Debugf("tank: %v", tank)
	asd.logger.Debugf("sz: %v", sz)

	level, ok := tank["Level"].(float64)
	if !ok {
		return fmt.Errorf("tank data has no level %v", tank)
	}

	flow, ok := sz["Product Water Flow"].(float64)
	if !ok {
		return fmt.Errorf("spotzero data has no flow %v", sz)
	}

	asd.logger.Infof("level %0.2f flow: %0.2f", level, flow)

	asd.mu.Lock()
	defer asd.mu.Unlock()
	if asd.res == nil {
		asd.res = map[string]interface{}{}
	}
	asd.res["level"] = level
	asd.res["flow"] = flow
	if level >= asd.conf.alertLevel() && flow > 0 {
		asd.res["fwerror"] = fmt.Sprintf("level %0.2f flow: %0.2f", level, flow)
	} else {
		asd.res["fwerror"] = ""
	}

	return nil
}

func (asd *AlertsSensorData) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	asd.mu.Lock()
	defer asd.mu.Unlock()
	return asd.res, nil
}

func (asd *AlertsSensorData) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (asd *AlertsSensorData) Close(ctx context.Context) error {
	asd.cancel()
	asd.wg.Wait()
	return nil
}

func (asd *AlertsSensorData) Name() resource.Name {
	return asd.name
}
