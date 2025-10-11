package verhboat

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.viam.com/rdk/logging"
)

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
	conf       *TahomaConfig
	httpClient *http.Client
	logger     logging.Logger
	devices    map[string]Device
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

func NewTahomaClient(conf *TahomaConfig, logger logging.Logger) (*TahomaClient, error) {
	tc := &TahomaClient{
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

func (c *TahomaClient) LowerAndTiltShadeByLabel(label string) error {
	d, ok := c.devices[label]
	if !ok {
		return fmt.Errorf("no device called [%s]", label)
	}
	return c.LowerAndTiltShadeByUrl(d.DeviceURL)

}

func (c *TahomaClient) LowerAndTiltShadeByUrl(deviceURL string) error {
	commands := []Command{
		{
			Name:       "down",
			Parameters: []interface{}{},
		},
	}

	_, err := c.ExecuteCommands(deviceURL, commands, "Lower shade")
	if err != nil {
		return fmt.Errorf("failed to lower shade: %w", err)
	}

	// Wait for the shade to fully lower
	time.Sleep(20 * time.Second)

	tiltCommands := []Command{
		{
			Name:       "tiltPositive",
			Parameters: []interface{}{8, 1}, // duration in 0.1s increments, unknown second param
		},
	}

	_, err = c.ExecuteCommands(deviceURL, tiltCommands, "Tilt shade up")
	if err != nil {
		return fmt.Errorf("failed to tilt shade: %w", err)
	}

	return nil
}
