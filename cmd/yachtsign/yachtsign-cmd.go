// Command yachtsign is a small CLI for testing the Nicolaudie STICK-DE3
// protocol used by the erh:verhboat:nicolaudie-stick3 component.
//
// It talks to the controller directly using the shared verhboat package, so
// it exercises exactly the same code the module runs.
//
// Examples:
//
//	go run ./cmd/yachtsign -ip 192.168.1.60 -action on -color 00FF88
//	go run ./cmd/yachtsign -ip 192.168.1.60 -action off
//	go run ./cmd/yachtsign -ip 192.168.1.60 -action color -color FF0000
//	go run ./cmd/yachtsign -action suntimes -lat 40.7128 -lng -74.0060
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/erh/verhboat"
)

func run() error {
	ip := flag.String("ip", "", "STICK-DE3 IPv4 address")
	port := flag.Int("port", verhboat.StickDefaultPort, "STICK-DE3 UDP quick-trigger port")
	action := flag.String("action", "", "action: on, off, pause, resume, reset, dimmer, dimmer-raw, speed, speed-raw, color, blackout, unblackout, or suntimes")
	pageValue := flag.String("page", "A", "page letter or zero-based page number; A=0, B=1")
	scene := flag.Int("scene", 1, "scene number within the page, from 1 to 50")
	value := flag.Int("value", 100, "percentage for dimmer/speed, or raw value for dimmer-raw/speed-raw")
	color := flag.String("color", "FFFFFF", "RGB color in RRGGBB format")
	lat := flag.Float64("lat", 0, "latitude in degrees (for suntimes)")
	lng := flag.Float64("lng", 0, "longitude in degrees, positive east (for suntimes)")
	timeout := flag.Duration("timeout", 2*time.Second, "UDP connection and write timeout")
	debug := flag.Bool("debug", false, "print the raw packet before sending")

	flag.Parse()

	act := strings.ToLower(strings.TrimSpace(*action))
	if act == "" {
		return errors.New("-action is required")
	}

	// suntimes doesn't touch the controller; handle it before requiring -ip.
	if act == "suntimes" {
		return printSunTimes(*lat, *lng)
	}

	if strings.TrimSpace(*ip) == "" {
		return errors.New("-ip is required")
	}

	page, err := verhboat.ParseStickPage(*pageValue)
	if err != nil {
		return err
	}

	client, err := verhboat.NewStick3Client(*ip, *port, *timeout, *debug)
	if err != nil {
		return err
	}

	switch act {
	case "on":
		// "on" both selects the scene and applies the requested color, matching
		// what the component does when it turns the sign on.
		red, green, blue, err := verhboat.ParseHexColor(*color)
		if err != nil {
			return err
		}
		if err := client.SceneOn(page, *scene); err != nil {
			return err
		}
		return client.SetColor(page, *scene, red, green, blue)

	case "off":
		return client.SceneOff(page, *scene)

	case "pause":
		return client.PauseScene(page, *scene)

	case "resume":
		return client.ResumeScene(page, *scene)

	case "reset":
		return client.ResetScene(page, *scene)

	case "dimmer":
		return client.SetDimmerPercent(page, *scene, *value)

	case "dimmer-raw":
		if *value < 0 || *value > 65535 {
			return errors.New("raw dimmer value must be between 0 and 65535")
		}
		return client.SetDimmerRaw(page, *scene, uint16(*value))

	case "speed":
		return client.SetSpeedPercent(page, *scene, *value)

	case "speed-raw":
		if *value < 0 || *value > 65535 {
			return errors.New("raw speed value must be between 0 and 65535")
		}
		return client.SetSpeedRaw(page, *scene, uint16(*value))

	case "color":
		red, green, blue, err := verhboat.ParseHexColor(*color)
		if err != nil {
			return err
		}
		return client.SetColor(page, *scene, red, green, blue)

	case "blackout":
		return client.BlackoutOn()

	case "unblackout":
		return client.BlackoutOff()

	default:
		return fmt.Errorf("unknown action %q", *action)
	}
}

func printSunTimes(lat, lng float64) error {
	now := time.Now().UTC()
	sunrise, sunset, err := verhboat.SunTimes(now, lat, lng)
	if err != nil {
		return err
	}
	fmt.Printf("location: %.4f, %.4f\n", lat, lng)
	fmt.Printf("sunrise (UTC):  %s\n", sunrise.Format(time.RFC3339))
	fmt.Printf("sunset  (UTC):  %s\n", sunset.Format(time.RFC3339))
	fmt.Printf("sunrise (local): %s\n", sunrise.Local().Format(time.RFC3339))
	fmt.Printf("sunset  (local): %s\n", sunset.Local().Format(time.RFC3339))
	fmt.Printf("sign on from %s until %s\n",
		sunset.Add(-time.Hour).Local().Format("15:04"),
		sunrise.Local().Format("15:04"))
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		fmt.Fprintln(os.Stderr)
		flag.Usage()
		os.Exit(1)
	}
}
