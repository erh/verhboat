package verhboat

// erh:verhboat:nicolaudie-stick3 drives a Nicolaudie STICK-DE3 lighting
// controller (e.g. a yacht name sign) over its UDP quick-trigger protocol.
//
// If a movement sensor (a GPS) is configured, the sign is turned on
// automatically one hour before sunset and off at sunrise, using the sun
// times at the sensor's current position. Without a movement sensor there is
// no schedule and the sign is controlled manually via DoCommand.
//
// Scheduling of color patterns is a planned follow-up; for now a single hex
// color is shown while the sign is on.

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var NicolaudieStick3Model = NamespaceFamily.WithModel("nicolaudie-stick3")

const (
	stick3SendTimeout   = 2 * time.Second
	stick3TickInterval  = 1 * time.Minute
	stick3SunsetLeadIn  = 1 * time.Hour // turn on this long before sunset
	stick3DefaultPageAt = "A"
	stick3DefaultScene  = 1
)

func init() {
	resource.RegisterComponent(
		generic.API,
		NicolaudieStick3Model,
		resource.Registration[resource.Resource, *NicolaudieStick3Config]{
			Constructor: newNicolaudieStick3,
		})
}

type NicolaudieStick3Config struct {
	IP   string `json:"ip"`
	Port int    `json:"port,omitempty"`

	// MovementSensor is the name of a movement sensor (GPS) used to get the
	// current location for sunrise/sunset scheduling. Optional; if empty, the
	// sign is only controlled manually.
	MovementSensor string `json:"movement_sensor,omitempty"`

	// Color is the hex color (RRGGBB) shown while the sign is on.
	Color string `json:"color"`

	// Page and Scene select the STICK scene that represents the sign. Page is
	// a letter (A, B, ...) or a zero-based number; Scene is 1-50. Defaults to
	// page A, scene 1.
	Page  string `json:"page,omitempty"`
	Scene int    `json:"scene,omitempty"`
}

func (c *NicolaudieStick3Config) Validate(path string) ([]string, []string, error) {
	if c.IP == "" {
		return nil, nil, fmt.Errorf("need an ip")
	}

	if _, err := ParseStickPage(c.pageOrDefault()); err != nil {
		return nil, nil, err
	}

	if c.Scene != 0 && (c.Scene < 1 || c.Scene > 50) {
		return nil, nil, fmt.Errorf("scene must be between 1 and 50, got %d", c.Scene)
	}

	if c.Color != "" {
		if _, _, _, err := ParseHexColor(c.Color); err != nil {
			return nil, nil, err
		}
	}

	var deps []string
	if c.MovementSensor != "" {
		deps = append(deps, c.MovementSensor)
	}
	return deps, nil, nil
}

func (c *NicolaudieStick3Config) pageOrDefault() string {
	if c.Page == "" {
		return stick3DefaultPageAt
	}
	return c.Page
}

func (c *NicolaudieStick3Config) sceneOrDefault() int {
	if c.Scene == 0 {
		return stick3DefaultScene
	}
	return c.Scene
}

type NicolaudieStick3 struct {
	resource.AlwaysRebuild

	name   resource.Name
	conf   *NicolaudieStick3Config
	logger logging.Logger

	client *Stick3Client
	page   int
	scene  int

	movement movementsensor.MovementSensor

	mu            sync.Mutex
	on            bool
	red, grn, blu byte

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newNicolaudieStick3(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (resource.Resource, error) {
	conf, err := resource.NativeConfig[*NicolaudieStick3Config](rawConf)
	if err != nil {
		return nil, err
	}

	port := conf.Port
	if port == 0 {
		port = StickDefaultPort
	}

	client, err := NewStick3Client(conf.IP, port, stick3SendTimeout, false)
	if err != nil {
		return nil, err
	}

	page, err := ParseStickPage(conf.pageOrDefault())
	if err != nil {
		return nil, err
	}

	s := &NicolaudieStick3{
		name:   rawConf.ResourceName(),
		conf:   conf,
		logger: logger,
		client: client,
		page:   page,
		scene:  conf.sceneOrDefault(),
	}

	if conf.Color != "" {
		s.red, s.grn, s.blu, _ = ParseHexColor(conf.Color)
	}

	if conf.MovementSensor != "" {
		s.movement, err = movementsensor.FromDependencies(deps, conf.MovementSensor)
		if err != nil {
			return nil, err
		}

		bgCtx, cancel := context.WithCancel(context.Background())
		s.cancel = cancel
		s.wg.Add(1)
		go s.scheduleLoop(bgCtx)
	}

	return s, nil
}

// setOn turns the sign on (scene on + configured color) or off. Caller need not
// hold the lock; setOn manages it.
func (s *NicolaudieStick3) setOn(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.setOnLocked(on)
}

func (s *NicolaudieStick3) setOnLocked(on bool) error {
	if on {
		if err := s.client.SceneOn(s.page, s.scene); err != nil {
			return err
		}
		if err := s.client.SetColor(s.page, s.scene, s.red, s.grn, s.blu); err != nil {
			return err
		}
	} else {
		if err := s.client.SceneOff(s.page, s.scene); err != nil {
			return err
		}
	}
	s.on = on
	return nil
}

// desiredOn reports whether the sign should be on right now, based on the sun
// times at the given location: on from one hour before sunset until sunrise.
func desiredOn(now time.Time, lat, lng float64) (bool, error) {
	sunrise, sunset, err := SunTimes(now, lat, lng)
	if err != nil {
		if err == ErrSunAlwaysDown {
			return true, nil // polar night: keep it on
		}
		if err == ErrSunAlwaysUp {
			return false, nil // polar day: keep it off
		}
		return false, err
	}

	onStart := sunset.Add(-stick3SunsetLeadIn)

	// On before this morning's sunrise (tail of last night), or after this
	// evening's lead-in to sunset.
	return now.Before(sunrise) || !now.Before(onStart), nil
}

func (s *NicolaudieStick3) evaluateSchedule(ctx context.Context) error {
	point, _, err := s.movement.Position(ctx, nil)
	if err != nil {
		return fmt.Errorf("reading position: %w", err)
	}

	want, err := desiredOn(time.Now().UTC(), point.Lat(), point.Lng())
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if want == s.on {
		return nil
	}
	s.logger.Infof("nicolaudie-stick3 %s: schedule turning %s", s.conf.IP, onOff(want))
	return s.setOnLocked(want)
}

func (s *NicolaudieStick3) scheduleLoop(ctx context.Context) {
	defer s.wg.Done()

	// Evaluate once at startup so the sign matches the schedule immediately.
	if err := s.evaluateSchedule(ctx); err != nil {
		s.logger.Warnf("nicolaudie-stick3 %s: initial schedule check failed: %v", s.conf.IP, err)
	}

	t := time.NewTicker(stick3TickInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.evaluateSchedule(ctx); err != nil {
				s.logger.Warnf("nicolaudie-stick3 %s: schedule check failed: %v", s.conf.IP, err)
			}
		}
	}
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func (s *NicolaudieStick3) Name() resource.Name {
	return s.name
}

func (s *NicolaudieStick3) Close(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	return nil
}

// DoCommand supports manual control and testing:
//
//	{"command": "on"}                          turn the sign on
//	{"command": "off"}                         turn the sign off
//	{"command": "set_color", "color": "FF0000"} change color (applied if on)
//	{"command": "status"}                      report current state
func (s *NicolaudieStick3) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	command, _ := cmd["command"].(string)

	switch command {
	case "on":
		if err := s.setOn(true); err != nil {
			return nil, err
		}
		return map[string]interface{}{"on": true}, nil

	case "off":
		if err := s.setOn(false); err != nil {
			return nil, err
		}
		return map[string]interface{}{"on": false}, nil

	case "set_color":
		hex, ok := cmd["color"].(string)
		if !ok {
			return nil, fmt.Errorf("set_color needs a string \"color\"")
		}
		red, grn, blu, err := ParseHexColor(hex)
		if err != nil {
			return nil, err
		}
		s.mu.Lock()
		s.red, s.grn, s.blu = red, grn, blu
		on := s.on
		var applyErr error
		if on {
			applyErr = s.client.SetColor(s.page, s.scene, red, grn, blu)
		}
		s.mu.Unlock()
		if applyErr != nil {
			return nil, applyErr
		}
		return map[string]interface{}{"color": hex}, nil

	case "status":
		s.mu.Lock()
		defer s.mu.Unlock()
		return map[string]interface{}{
			"on":    s.on,
			"color": fmt.Sprintf("%02X%02X%02X", s.red, s.grn, s.blu),
		}, nil

	default:
		return nil, fmt.Errorf("unknown command %q", command)
	}
}
