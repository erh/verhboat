package verhboat

// Nicolaudie STICK-DE3 UDP "Quick Trigger" remote protocol.
//
// This is the low-level wire protocol, extracted so both the
// erh:verhboat:nicolaudie-stick3 component and the cmd/yachtsign CLI can
// share it. See nicolaudie_stick3.go for the Viam component.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	// StickDefaultPort is the STICK-DE3 UDP quick-trigger port.
	StickDefaultPort = 2430

	stickDeviceID           = "Stick_3A"
	stickPacketSize         = 24
	stickQuickTriggerOpcode = uint16(109)
)

// StickCommand is the command byte in a quick-trigger packet.
type StickCommand byte

const (
	StickSceneOff    StickCommand = 0
	StickSceneOn     StickCommand = 1
	StickPauseOff    StickCommand = 2
	StickPauseOn     StickCommand = 3
	StickSceneReset  StickCommand = 4
	StickDimmerSet   StickCommand = 5
	StickSpeedSet    StickCommand = 6
	StickColorSet    StickCommand = 7
	StickBlackoutOff StickCommand = 8
	StickBlackoutOn  StickCommand = 9
)

// Stick3Client is a STICK-DE3 UDP quick-trigger client.
type Stick3Client struct {
	address string
	timeout time.Duration
	debug   bool
}

// StickQuickTrigger is a single quick-trigger command.
type StickQuickTrigger struct {
	Scene      uint16
	ZoneSyncID byte
	Command    StickCommand
	Dimmer     uint16
	Speed      uint16
	Red        byte
	Green      byte
	Blue       byte
}

// NewStick3Client creates a STICK-DE3 UDP client.
func NewStick3Client(ip string, port int, timeout time.Duration, debug bool) (*Stick3Client, error) {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return nil, fmt.Errorf("invalid controller IP address %q", ip)
	}

	if port < 1 || port > 65535 {
		return nil, fmt.Errorf("invalid UDP port %d", port)
	}

	if timeout <= 0 {
		return nil, errors.New("timeout must be greater than zero")
	}

	return &Stick3Client{
		address: net.JoinHostPort(ip, strconv.Itoa(port)),
		timeout: timeout,
		debug:   debug,
	}, nil
}

// StickAbsoluteScene converts a zero-based page and a one-based scene number
// into the absolute scene number expected by the STICK-DE3.
//
// Page A = 0, Page B = 1, ...
//
// Absolute scene = page*50 + scene
func StickAbsoluteScene(page int, scene int) (uint16, error) {
	if page < 0 {
		return 0, errors.New("page cannot be negative")
	}

	if scene < 1 || scene > 50 {
		return 0, errors.New("scene must be between 1 and 50")
	}

	absolute := page*50 + scene
	if absolute > 65535 {
		return 0, errors.New("absolute scene number exceeds 65535")
	}

	return uint16(absolute), nil
}

// BuildStickQuickTrigger builds the documented 24-byte STICK-DE3 packet.
//
// Packet layout:
//
//	0-7    "Stick_3A"
//	8-9    opcode 109
//	10-11  absolute scene number
//	12     zone synchronization ID
//	13     command
//	14-15  dimmer value
//	16-17  speed value
//	18-19  unused/alignment
//	20     red
//	21     green
//	22     blue
//	23     unused color byte
//
// Nicolaudie's examples encode uint16 values least-significant byte first,
// such as opcode 109 being encoded as 6D 00.
func BuildStickQuickTrigger(trigger StickQuickTrigger) []byte {
	packet := make([]byte, stickPacketSize)

	copy(packet[0:8], []byte(stickDeviceID))

	binary.LittleEndian.PutUint16(packet[8:10], stickQuickTriggerOpcode)
	binary.LittleEndian.PutUint16(packet[10:12], trigger.Scene)

	packet[12] = trigger.ZoneSyncID
	packet[13] = byte(trigger.Command)

	binary.LittleEndian.PutUint16(packet[14:16], trigger.Dimmer)
	binary.LittleEndian.PutUint16(packet[16:18], trigger.Speed)

	packet[18] = 0
	packet[19] = 0

	packet[20] = trigger.Red
	packet[21] = trigger.Green
	packet[22] = trigger.Blue
	packet[23] = 0

	return packet
}

// Send transmits a single quick-trigger command over UDP.
func (c *Stick3Client) Send(trigger StickQuickTrigger) error {
	packet := BuildStickQuickTrigger(trigger)

	if c.debug {
		fmt.Printf("destination: %s\n", c.address)
		fmt.Printf("packet size: %d\n", len(packet))
		fmt.Printf("packet: % X\n", packet)
	}

	conn, err := net.DialTimeout("udp", c.address, c.timeout)
	if err != nil {
		return fmt.Errorf("open UDP connection to %s: %w", c.address, err)
	}
	defer conn.Close()

	if err := conn.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return fmt.Errorf("set UDP write deadline: %w", err)
	}

	written, err := conn.Write(packet)
	if err != nil {
		return fmt.Errorf("send UDP packet: %w", err)
	}

	if written != len(packet) {
		return fmt.Errorf("short UDP write: wrote %d bytes, expected %d", written, len(packet))
	}

	return nil
}

func (c *Stick3Client) SceneOn(page int, scene int) error {
	absolute, err := StickAbsoluteScene(page, scene)
	if err != nil {
		return err
	}
	return c.Send(StickQuickTrigger{Scene: absolute, Command: StickSceneOn})
}

func (c *Stick3Client) SceneOff(page int, scene int) error {
	absolute, err := StickAbsoluteScene(page, scene)
	if err != nil {
		return err
	}
	return c.Send(StickQuickTrigger{Scene: absolute, Command: StickSceneOff})
}

func (c *Stick3Client) PauseScene(page int, scene int) error {
	absolute, err := StickAbsoluteScene(page, scene)
	if err != nil {
		return err
	}
	return c.Send(StickQuickTrigger{Scene: absolute, Command: StickPauseOn})
}

func (c *Stick3Client) ResumeScene(page int, scene int) error {
	absolute, err := StickAbsoluteScene(page, scene)
	if err != nil {
		return err
	}
	return c.Send(StickQuickTrigger{Scene: absolute, Command: StickPauseOff})
}

func (c *Stick3Client) ResetScene(page int, scene int) error {
	absolute, err := StickAbsoluteScene(page, scene)
	if err != nil {
		return err
	}
	return c.Send(StickQuickTrigger{Scene: absolute, Command: StickSceneReset})
}

// SetDimmerRaw sends the raw STICK dimmer value (0 = 0%, 127 = 100%, 255 = 200%).
func (c *Stick3Client) SetDimmerRaw(page int, scene int, value uint16) error {
	absolute, err := StickAbsoluteScene(page, scene)
	if err != nil {
		return err
	}
	return c.Send(StickQuickTrigger{Scene: absolute, Command: StickDimmerSet, Dimmer: value})
}

// SetDimmerPercent maps 0-100% to the documented raw range 0-127.
func (c *Stick3Client) SetDimmerPercent(page int, scene int, percent int) error {
	if percent < 0 || percent > 100 {
		return errors.New("dimmer percentage must be between 0 and 100")
	}
	raw := uint16((percent*127 + 50) / 100)
	return c.SetDimmerRaw(page, scene, raw)
}

// SetSpeedRaw sends the raw STICK speed value (0 = minimum, 127 = 100%, 255 = 600%).
func (c *Stick3Client) SetSpeedRaw(page int, scene int, value uint16) error {
	absolute, err := StickAbsoluteScene(page, scene)
	if err != nil {
		return err
	}
	return c.Send(StickQuickTrigger{Scene: absolute, Command: StickSpeedSet, Speed: value})
}

// SetSpeedPercent maps 0-100% to the documented raw range 0-127.
func (c *Stick3Client) SetSpeedPercent(page int, scene int, percent int) error {
	if percent < 0 || percent > 100 {
		return errors.New("speed percentage must be between 0 and 100")
	}
	raw := uint16((percent*127 + 50) / 100)
	return c.SetSpeedRaw(page, scene, raw)
}

func (c *Stick3Client) SetColor(page int, scene int, red, green, blue byte) error {
	absolute, err := StickAbsoluteScene(page, scene)
	if err != nil {
		return err
	}
	return c.Send(StickQuickTrigger{
		Scene:   absolute,
		Command: StickColorSet,
		Red:     red,
		Green:   green,
		Blue:    blue,
	})
}

func (c *Stick3Client) BlackoutOn() error {
	return c.Send(StickQuickTrigger{Scene: 0, Command: StickBlackoutOn})
}

func (c *Stick3Client) BlackoutOff() error {
	return c.Send(StickQuickTrigger{Scene: 0, Command: StickBlackoutOff})
}

// ParseStickPage parses a page letter (A=0, B=1, ...) or a zero-based page
// number into a zero-based page index.
func ParseStickPage(value string) (int, error) {
	value = strings.TrimSpace(strings.ToUpper(value))

	if value == "" {
		return 0, errors.New("page cannot be empty")
	}

	if len(value) == 1 && value[0] >= 'A' && value[0] <= 'Z' {
		return int(value[0] - 'A'), nil
	}

	page, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid page %q; use a letter such as A or B, or a zero-based number", value)
	}

	if page < 0 {
		return 0, errors.New("page cannot be negative")
	}

	return page, nil
}

// ParseHexColor parses an RRGGBB hex color (with or without a leading '#')
// into red, green, and blue bytes.
func ParseHexColor(value string) (red, green, blue byte, err error) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "#")

	if len(value) != 6 {
		return 0, 0, 0, fmt.Errorf("invalid hex color %q; expected six hex digits such as FF0000", value)
	}

	parsed, err := strconv.ParseUint(value, 16, 24)
	if err != nil {
		return 0, 0, 0, fmt.Errorf("parse hex color %q: %w", value, err)
	}

	return byte((parsed >> 16) & 0xff), byte((parsed >> 8) & 0xff), byte(parsed & 0xff), nil
}
