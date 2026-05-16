package verhboat

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"regexp"
	"strconv"
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
	m4315SyncInterval   = 5 * time.Minute
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

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func newM4315Pro(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (toggleswitch.Switch, error) {
	conf, err := resource.NativeConfig[*M4315ProConfig](rawConf)
	if err != nil {
		return nil, err
	}

	bgCtx, cancel := context.WithCancel(context.Background())
	s := &M4315Pro{
		name:   rawConf.ResourceName(),
		conf:   conf,
		logger: logger,
		cancel: cancel,
	}

	if err := s.syncFromDevice(ctx); err != nil {
		// Don't fail startup; the device may be temporarily unreachable.
		// The background poller will retry every m4315SyncInterval.
		logger.Warnf("m4315-pro %s outlet %d: initial status query failed: %v",
			conf.Host, conf.Outlet, err)
	}

	s.wg.Add(1)
	go s.syncLoop(bgCtx)

	return s, nil
}

func (s *M4315Pro) tcpPort() int {
	if s.conf.TCPPort == 0 {
		return m4315DefaultTCPPort
	}
	return s.conf.TCPPort
}

// dialAndAuth opens a fresh telnet connection and, if a password is configured,
// logs in. The returned bufio.Reader is positioned past the login prompt.
func (s *M4315Pro) dialAndAuth() (net.Conn, *bufio.Reader, error) {
	addr := net.JoinHostPort(s.conf.Host, strconv.Itoa(s.tcpPort()))
	conn, err := net.DialTimeout("tcp", addr, m4315DialTimeout)
	if err != nil {
		return nil, nil, fmt.Errorf("dial %s: %w", addr, err)
	}
	_ = conn.SetDeadline(time.Now().Add(m4315IOTimeout))

	reader := bufio.NewReader(conn)

	if s.conf.Password != "" {
		if err := readUntilPrompt(reader, "password"); err != nil {
			conn.Close()
			return nil, nil, fmt.Errorf("waiting for password prompt: %w", err)
		}
		if _, err := conn.Write([]byte(s.conf.Password + "\r")); err != nil {
			conn.Close()
			return nil, nil, fmt.Errorf("sending password: %w", err)
		}
		if err := readUntilPrompt(reader, ">"); err != nil {
			conn.Close()
			return nil, nil, fmt.Errorf("waiting for command prompt: %w", err)
		}
	}

	return conn, reader, nil
}

// sendSwitch sends one !SWITCH command for the configured outlet.
func (s *M4315Pro) sendSwitch(state string) error {
	conn, _, err := s.dialAndAuth()
	if err != nil {
		return err
	}
	defer conn.Close()

	cmd := fmt.Sprintf("!SWITCH %d %s\r", s.conf.Outlet, state)
	s.logger.Debugf("m4315-pro %s outlet %d -> %s", s.conf.Host, s.conf.Outlet, state)
	if _, err := conn.Write([]byte(cmd)); err != nil {
		return fmt.Errorf("sending command: %w", err)
	}
	return nil
}

// outletStatusRE matches one outlet line in a ?OUTLETSTAT response, e.g.
// "$OUTLET1 ON", "$OUTLET1=ON", "$OUTLET1 = OFF".
var outletStatusRE = regexp.MustCompile(`(?i)\$OUTLET\s*(\d+)\s*[=: ]\s*(ON|OFF)`)

// queryStatus sends ?OUTLETSTAT and returns the on/off state of the configured
// outlet (true = on).
func (s *M4315Pro) queryStatus() (bool, error) {
	conn, reader, err := s.dialAndAuth()
	if err != nil {
		return false, err
	}
	defer conn.Close()

	if _, err := conn.Write([]byte("?OUTLETSTAT\r")); err != nil {
		return false, fmt.Errorf("sending query: %w", err)
	}

	// Read until we've seen our outlet's status or the deadline fires. The
	// device emits one $OUTLETn line per outlet; we don't know exactly how
	// many lines it will send, so we read until the deadline ends the
	// connection and parse what we got.
	var buf strings.Builder
	tmp := make([]byte, 256)
	for {
		n, err := reader.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
			// Fast-path: if we've already seen our outlet, stop reading.
			if state, ok := parseOutletStatus(buf.String(), s.conf.Outlet); ok {
				return state, nil
			}
		}
		if err != nil {
			// Connection closed or deadline hit; try a final parse.
			if state, ok := parseOutletStatus(buf.String(), s.conf.Outlet); ok {
				return state, nil
			}
			return false, fmt.Errorf("reading status: %w (got %q)", err, buf.String())
		}
	}
}

// parseOutletStatus scans device output for the configured outlet's state.
func parseOutletStatus(text string, outlet int) (bool, bool) {
	for _, m := range outletStatusRE.FindAllStringSubmatch(text, -1) {
		n, err := strconv.Atoi(m[1])
		if err != nil || n != outlet {
			continue
		}
		return strings.EqualFold(m[2], "ON"), true
	}
	return false, false
}

// syncFromDevice queries the device and updates the cached position.
func (s *M4315Pro) syncFromDevice(ctx context.Context) error {
	on, err := s.queryStatus()
	if err != nil {
		return err
	}
	var pos uint32
	if on {
		pos = 1
	}
	s.mu.Lock()
	if s.lastPosition != pos {
		s.logger.Infof("m4315-pro %s outlet %d: syncing cached state %d -> %d",
			s.conf.Host, s.conf.Outlet, s.lastPosition, pos)
	}
	s.lastPosition = pos
	s.mu.Unlock()
	return nil
}

func (s *M4315Pro) syncLoop(ctx context.Context) {
	defer s.wg.Done()
	t := time.NewTicker(m4315SyncInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := s.syncFromDevice(ctx); err != nil {
				s.logger.Warnf("m4315-pro %s outlet %d: status sync failed: %v",
					s.conf.Host, s.conf.Outlet, err)
			}
		}
	}
}

// readUntilPrompt reads from r until the accumulated input contains substr
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
	s.cancel()
	s.wg.Wait()
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
