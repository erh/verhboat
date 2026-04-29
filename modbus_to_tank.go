package verhboat

import (
	"context"
	"fmt"

	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var ModbusToTankSensorModel = NamespaceFamily.WithModel("modbus-to-tank")

func init() {
	resource.RegisterComponent(
		sensor.API,
		ModbusToTankSensorModel,
		resource.Registration[sensor.Sensor, *ModbusToTankSensorConfig]{
			Constructor: newModbusToTankSensor,
		})
}

type ModbusToTankSensorConfig struct {
	ModbusSensor string  `json:"modbus-sensor"`
	Capacity     float64 `json:"capacity"`
	Type         string  `json:"type"`
	Field        string  `json:"field"`
}

func (c *ModbusToTankSensorConfig) Validate(_ string) ([]string, []string, error) {
	if c.ModbusSensor == "" {
		return nil, nil, fmt.Errorf("need modbus-sensor")
	}
	if c.Capacity <= 0 {
		return nil, nil, fmt.Errorf("need capacity > 0")
	}
	if c.Type == "" {
		return nil, nil, fmt.Errorf("need type")
	}
	if c.Field == "" {
		return nil, nil, fmt.Errorf("need field")
	}
	return []string{c.ModbusSensor}, nil, nil
}

func newModbusToTankSensor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (sensor.Sensor, error) {
	conf, err := resource.NativeConfig[*ModbusToTankSensorConfig](rawConf)
	if err != nil {
		return nil, err
	}

	d := &ModbusToTankSensorData{
		name:   rawConf.ResourceName(),
		logger: logger,
		conf:   conf,
	}

	d.modbusSensor, err = sensor.FromDependencies(deps, conf.ModbusSensor)
	if err != nil {
		return nil, err
	}

	return d, nil
}

type ModbusToTankSensorData struct {
	resource.AlwaysRebuild

	name   resource.Name
	conf   *ModbusToTankSensorConfig
	logger logging.Logger

	modbusSensor sensor.Sensor
}

func (m *ModbusToTankSensorData) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	res, err := m.modbusSensor.Readings(ctx, extra)
	if err != nil {
		return nil, fmt.Errorf("can't read from modbus-sensor: %w", err)
	}

	rawAny, ok := res[m.conf.Field]
	if !ok {
		return nil, fmt.Errorf("modbus-sensor has no field %q, got %v", m.conf.Field, res)
	}

	raw, ok := rawAny.(float64)
	if !ok {
		return nil, fmt.Errorf("modbus-sensor field %q is not a float64: %v (%)", m.conf.Field, rawAny, rawAny)
	}

	const gallonsToLiters = 3.78541
	raw = (raw / 10) * gallonsToLiters
	cap := m.conf.Capacity * gallonsToLiters

	return map[string]interface{}{
		"raw":      raw,
		"Capacity": cap,
		"Type":     m.conf.Type,
		"Level":    (raw / cap) * 100,
	}, nil
}

func (m *ModbusToTankSensorData) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (m *ModbusToTankSensorData) Close(ctx context.Context) error {
	return nil
}

func (m *ModbusToTankSensorData) Name() resource.Name {
	return m.name
}
