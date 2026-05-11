package verhboat

import (
	"context"
	"fmt"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var CombinedTankSensorModel = NamespaceFamily.WithModel("combined-tank")

func init() {
	resource.RegisterComponent(
		sensor.API,
		CombinedTankSensorModel,
		resource.Registration[sensor.Sensor, *CombinedTankSensorConfig]{
			Constructor: newCombinedTankSensor,
		})
}

type CombinedTankSensorConfig struct {
	Tanks []string `json:"tanks"`
}

func (c *CombinedTankSensorConfig) Validate(_ string) ([]string, []string, error) {
	if len(c.Tanks) == 0 {
		return nil, nil, fmt.Errorf("need at least one tank")
	}
	for _, t := range c.Tanks {
		if t == "" {
			return nil, nil, fmt.Errorf("tank name cannot be empty")
		}
	}
	return c.Tanks, nil, nil
}

func newCombinedTankSensor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (sensor.Sensor, error) {
	conf, err := resource.NativeConfig[*CombinedTankSensorConfig](rawConf)
	if err != nil {
		return nil, err
	}

	d := &CombinedTankSensorData{
		name:   rawConf.ResourceName(),
		logger: logger,
		conf:   conf,
	}

	for _, t := range conf.Tanks {
		s, err := sensor.FromDependencies(deps, t)
		if err != nil {
			return nil, err
		}
		d.tanks = append(d.tanks, s)
	}

	return d, nil
}

type CombinedTankSensorData struct {
	resource.AlwaysRebuild

	name   resource.Name
	conf   *CombinedTankSensorConfig
	logger logging.Logger

	tanks []sensor.Sensor
}

func (m *CombinedTankSensorData) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	var totalCapacity, totalLiters float64
	var tankType string

	for i, t := range m.tanks {
		res, err := t.Readings(ctx, extra)
		if err != nil {
			return nil, fmt.Errorf("can't read from tank %q: %w", m.conf.Tanks[i], err)
		}

		capacity, ok := res["Capacity"].(float64)
		if !ok {
			return nil, fmt.Errorf("tank %q has no float64 \"Capacity\": %v", m.conf.Tanks[i], res["Capacity"])
		}
		liters, ok := res["Liters"].(float64)
		if !ok {
			return nil, fmt.Errorf("tank %q has no float64 \"Liters\": %v", m.conf.Tanks[i], res["Liters"])
		}
		typ, ok := res["Type"].(string)
		if !ok {
			return nil, fmt.Errorf("tank %q has no string \"Type\": %v", m.conf.Tanks[i], res["Type"])
		}

		if tankType == "" {
			tankType = typ
		} else if tankType != typ {
			return nil, fmt.Errorf("tank %q has type %q but expected %q", m.conf.Tanks[i], typ, tankType)
		}

		totalCapacity += capacity
		totalLiters += liters
	}

	level := 0.0
	if totalCapacity > 0 {
		level = (totalLiters / totalCapacity) * 100
	}

	return map[string]interface{}{
		"Capacity": totalCapacity,
		"Type":     tankType,
		"Level":    level,
		"Liters":   totalLiters,
	}, nil
}

func (m *CombinedTankSensorData) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (m *CombinedTankSensorData) Close(ctx context.Context) error {
	return nil
}

func (m *CombinedTankSensorData) Name() resource.Name {
	return m.name
}
