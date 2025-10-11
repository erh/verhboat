package verhboat

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/multierr"

	"go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var TahomaHackModel = NamespaceFamily.WithModel("tahoma-hack")

func init() {
	resource.RegisterComponent(
		toggleswitch.API,
		TahomaHackModel,
		resource.Registration[toggleswitch.Switch, *TahomaConfig]{
			Constructor: newTahomaHack,
		})
}

type TahomaConfig struct {
	Host   string
	ApiKey string `json:"api-key"`
}

func (tc *TahomaConfig) Validate(path string) ([]string, []string, error) {
	if tc.Host == "" {
		return nil, nil, fmt.Errorf("need a host")
	}

	if tc.ApiKey == "" {
		return nil, nil, fmt.Errorf("need an api-key")
	}

	return []string{}, nil, nil
}

type TahomaClient struct {
	resource.AlwaysRebuild

	name   resource.Name
	conf   *TahomaConfig
	logger logging.Logger

	httpClient *http.Client

	devices      map[string]Device
	lastPosition uint32
}

// ------
// these are all part of the API

type Device struct {
	DeviceURL string `json:"deviceURL"`
	Label     string `json:"label"`
}

type Command struct {
	Name       string        `json:"name"`
	Parameters []interface{} `json:"parameters"`
}

type Action struct {
	DeviceURL string    `json:"deviceURL"`
	Commands  []Command `json:"commands"`
}

type ExecutionRequest struct {
	Label   string   `json:"label"`
	Actions []Action `json:"actions"`
}

type ExecutionResponse struct {
	ExecID string `json:"execId"`
}

// --- end api ---

func newTahomaHack(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (toggleswitch.Switch, error) {
	conf, err := resource.NativeConfig[*TahomaConfig](rawConf)
	if err != nil {
		return nil, err
	}

	return NewTahomaClient(conf, rawConf.ResourceName(), logger)
}

func NewTahomaClient(conf *TahomaConfig, name resource.Name, logger logging.Logger) (*TahomaClient, error) {
	tc := &TahomaClient{
		name: name,
		conf: conf,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
				DisableKeepAlives: true,
			},
		},
		devices: map[string]Device{},
		logger:  logger,
	}

	devices, err := tc.GetDevices()
	if err != nil {
		return nil, err
	}

	for _, d := range devices {
		tc.devices[d.Label] = d
	}

	return tc, nil
}

func (c *TahomaClient) makeRequest(method, endpoint string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, fmt.Sprintf("https://%s:8443/enduser-mobile-web/1/enduserAPI%s", c.conf.Host, endpoint), reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.conf.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

func (c *TahomaClient) GetDevices() ([]Device, error) {
	respBody, err := c.makeRequest("GET", "/setup/devices", nil)
	if err != nil {
		return nil, err
	}

	//fmt.Printf("%s\n", string(respBody)) // TODO - add more things to Device

	var devices []Device
	if err := json.Unmarshal(respBody, &devices); err != nil {
		return nil, fmt.Errorf("failed to parse devices: %w", err)
	}

	return devices, nil
}

func (c *TahomaClient) ExecuteCommands(deviceURL string, commands []Command, label string) (string, error) {
	execReq := ExecutionRequest{
		Label: label,
		Actions: []Action{
			{
				DeviceURL: deviceURL,
				Commands:  commands,
			},
		},
	}

	respBody, err := c.makeRequest("POST", "/exec/apply", execReq)
	if err != nil {
		return "", err
	}

	var execResp ExecutionResponse
	if err := json.Unmarshal(respBody, &execResp); err != nil {
		return "", fmt.Errorf("failed to parse execution response: %w", err)
	}

	c.logger.Debugf("result %v", execResp)

	return execResp.ExecID, nil
}

func (c *TahomaClient) LiftShadeByLabel(label string) error {
	d, ok := c.devices[label]
	if !ok {
		return fmt.Errorf("no device called [%s]", label)
	}
	return c.LiftShadeByUrl(d.DeviceURL)
}

func (c *TahomaClient) LiftShadeByUrl(deviceURL string) error {
	commands := []Command{
		{
			Name:       "up",
			Parameters: []interface{}{},
		},
	}

	_, err := c.ExecuteCommands(deviceURL, commands, "Raise shade")
	return err
}

func (c *TahomaClient) LowerAndTiltShadeByLabels(labels []string) error {
	urls, err := c.labelsToUrls(labels)
	if err != nil {
		return err
	}
	return c.LowerAndTiltShadeByUrls(urls)

}

func (c *TahomaClient) labelsToUrls(labels []string) ([]string, error) {
	urls := []string{}

	for _, l := range labels {
		d, ok := c.devices[l]
		if !ok {
			return nil, fmt.Errorf("no device called [%s]", l)
		}
		urls = append(urls, d.DeviceURL)
	}

	return urls, nil
}

func (c *TahomaClient) LowerAndTiltShadeByUrls(urls []string) error {
	commands := []Command{
		{
			Name:       "down",
			Parameters: []interface{}{},
		},
	}

	for _, u := range urls {
		_, err := c.ExecuteCommands(u, commands, "Lower shade")
		if err != nil {
			return fmt.Errorf("failed to lower shade (url: %s): %w", u, err)
		}
	}

	time.Sleep(20 * time.Second) // Wait for the shade(s) to fully lower

	tiltCommands := []Command{
		{
			Name:       "tiltPositive",
			Parameters: []interface{}{8, 1}, // duration in 0.1s increments, unknown second param
		},
	}

	for _, u := range urls {
		_, err := c.ExecuteCommands(u, tiltCommands, "Tilt shade up")
		if err != nil {
			return fmt.Errorf("failed to tilt shade (url: %s): %w", u, err)
		}
	}

	return nil
}

func (c *TahomaClient) Close(ctx context.Context) error {
	c.httpClient.CloseIdleConnections()
	return nil
}

func (c *TahomaClient) Name() resource.Name {
	return c.name
}

func (c *TahomaClient) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

func (c *TahomaClient) SetPosition(ctx context.Context, position uint32, extra map[string]interface{}) error {
	// TODO - this is all hacked for now

	switch position {
	case 0:
		c.lastPosition = position
		return nil
	case 1:
		err := multierr.Combine(
			c.LiftShadeByLabel("Port forward"),
			c.LiftShadeByLabel("Port mid"),
		)
		if err != nil {
			c.lastPosition = 0
			return err
		}
		c.lastPosition = position
		return nil
	case 2:
		err := c.LowerAndTiltShadeByLabels([]string{"Port forward", "Port mid"})
		if err != nil {
			c.lastPosition = 0
			return err
		}
		c.lastPosition = position
		return nil
	}

	return fmt.Errorf("don't know how to go to position %d", position)
}

func (c *TahomaClient) GetPosition(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	return c.lastPosition, nil
}

func (c *TahomaClient) GetNumberOfPositions(ctx context.Context, extra map[string]interface{}) (uint32, []string, error) {
	return 3, []string{"unknown", "open", "port"}, nil
}
