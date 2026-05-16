package verhboat

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var M4315ProModel = NamespaceFamily.WithModel("m4315-pro")

const (
	m4315DefaultTCPPort = 23
	m4315DialTimeout    = 5 * time.Second
	m4315IOTimeout      = 5 * time.Second
)

func init() {
	resource.RegisterComponent(
		toggleswitch.API,
		M4315ProModel,
		resource.Registration[toggleswitch.Switch, *M4315ProConfig]{
			Constructor: newM4315Pro,
		})
}

type M4315ProConfig struct {
	Host     string `json:"host"`
	TCPPort  int    `json:"tcp-port,omitempty"`
	Outlet   int    `json:"outlet"`
	Password string `json:"password,omitempty"`
}

func (c *M4315ProConfig) Validate(path string) ([]string, []string, error) {
	if c.Host == "" {
		return nil, nil, fmt.Errorf("need a host")
	}
	if c.Outlet < 1 || c.Outlet > 8 {
		return nil, nil, fmt.Errorf("outlet must be between 1 and 8, got %d", c.Outlet)
	}
	return nil, nil, nil
}

type M4315Pro struct {
	resource.AlwaysRebuild

	name   resource.Name
	conf   *M4315ProConfig
	logger logging.Logger

	mu           sync.Mutex
	lastPosition uint32
}

func newM4315Pro(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (toggleswitch.Switch, error) {
	conf, err := resource.NativeConfig[*M4315ProConfig](rawConf)
	if err != nil {
		return nil, err
	}
	return &M4315Pro{
		name:   rawConf.ResourceName(),
		conf:   conf,
		logger: logger,
	}, nil
}

func (s *M4315Pro) tcpPort() int {
	if s.conf.TCPPort == 0 {
		return m4315DefaultTCPPort
	}
	return s.conf.TCPPort
}

// sendSwitch opens a fresh telnet connection, optionally logs in, and sends
// one !SWITCH command for the configured outlet.
func (s *M4315Pro) sendSwitch(state string) error {
	addr := net.JoinHostPort(s.conf.Host, fmt.Sprintf("%d", s.tcpPort()))
	conn, err := net.DialTimeout("tcp", addr, m4315DialTimeout)
	if err != nil {
		return fmt.Errorf("dial %s: %w", addr, err)
	}
	defer conn.Close()

	deadline := time.Now().Add(m4315IOTimeout)
	_ = conn.SetDeadline(deadline)

	reader := bufio.NewReader(conn)

	if s.conf.Password != "" {
		// The BlueBOLT-CV1 card prompts for the password before accepting
		// commands. Read until we see a prompt that looks like one, then
		// send the password.
		if err := readUntilPrompt(reader, "password"); err != nil {
			return fmt.Errorf("waiting for password prompt: %w", err)
		}
		if _, err := conn.Write([]byte(s.conf.Password + "\r")); err != nil {
			return fmt.Errorf("sending password: %w", err)
		}
		if err := readUntilPrompt(reader, ">"); err != nil {
			return fmt.Errorf("waiting for command prompt: %w", err)
		}
	}

	cmd := fmt.Sprintf("!SWITCH %d %s\r", s.conf.Outlet, state)
	s.logger.Debugf("m4315-pro %s outlet %d -> %s", s.conf.Host, s.conf.Outlet, state)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return fmt.Errorf("sending command: %w", err)
	}

	return nil
}

// readUntilPrompt reads from r until the accumulated input ends with substr
// (case-insensitive), or the connection deadline fires.
func readUntilPrompt(r *bufio.Reader, substr string) error {
	want := strings.ToLower(substr)
	var buf strings.Builder
	for {
		b, err := r.ReadByte()
		if err != nil {
			return err
		}
		buf.WriteByte(b)
		if strings.Contains(strings.ToLower(buf.String()), want) {
			return nil
		}
	}
}

func (s *M4315Pro) Name() resource.Name {
	return s.name
}

func (s *M4315Pro) Close(ctx context.Context) error {
	return nil
}

func (s *M4315Pro) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (s *M4315Pro) SetPosition(ctx context.Context, position uint32, extra map[string]interface{}) error {
	var state string
	switch position {
	case 0:
		state = "OFF"
	case 1:
		state = "ON"
	default:
		return fmt.Errorf("m4315-pro only supports positions 0 (off) and 1 (on), got %d", position)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.sendSwitch(state); err != nil {
		return err
	}
	s.lastPosition = position
	return nil
}

func (s *M4315Pro) GetPosition(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastPosition, nil
}

func (s *M4315Pro) GetNumberOfPositions(ctx context.Context, extra map[string]interface{}) (uint32, []string, error) {
	return 2, []string{"off", "on"}, nil
}
