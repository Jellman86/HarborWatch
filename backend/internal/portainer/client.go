package portainer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Stack represents a Portainer stack.
type Stack struct {
	ID         int    `json:"Id"`
	Name       string `json:"Name"`
	Type       int    `json:"Type"`
	EndpointID int    `json:"EndpointId"`
	SwarmID    string `json:"SwarmId"`
	EntryPoint string `json:"EntryPoint"`
	Status     int    `json:"Status"`
}

// Endpoint represents a Portainer endpoint (Environment).
type Endpoint struct {
	ID      int    `json:"Id"`
	Name    string `json:"Name"`
	Type    int    `json:"Type"`
	URL     string `json:"URL"`
	Status  int    `json:"Status"`
}

// Client is a Portainer API client.
type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) ListEndpoints(ctx context.Context) ([]Endpoint, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/endpoints", c.baseURL), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("portainer api error: status %d", resp.StatusCode)
	}

	var endpoints []Endpoint
	if err := json.NewDecoder(resp.Body).Decode(&endpoints); err != nil {
		return nil, err
	}

	return endpoints, nil
}

func (c *Client) ListStacks(ctx context.Context) ([]Stack, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/stacks", c.baseURL), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("portainer api error: status %d", resp.StatusCode)
	}

	var stacks []Stack
	if err := json.NewDecoder(resp.Body).Decode(&stacks); err != nil {
		return nil, err
	}

	return stacks, nil
}

func (c *Client) GetStackFile(ctx context.Context, stackID int) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/stacks/%d/file", c.baseURL, stackID), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("portainer api error: status %d", resp.StatusCode)
	}

	var data struct {
		StackFileContent string `json:"StackFileContent"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	return data.StackFileContent, nil
}

func (c *Client) UpdateStack(ctx context.Context, stackID int, endpointID int, yaml string, env []map[string]string, prune bool, pullImage bool) error {
	payload := map[string]any{
		"StackFileContent": yaml,
		"Env":              env,
		"Prune":            prune,
		"PullImage":        pullImage,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/api/stacks/%d?endpointId=%d", c.baseURL, stackID, endpointID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, strings.NewReader(string(data)))
	if err != nil {
		return err
	}

	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("portainer api error: status %d", resp.StatusCode)
	}

	return nil
}

func (c *Client) GetStack(ctx context.Context, stackID int) (*Stack, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/stacks/%d", c.baseURL, stackID), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("portainer api error: status %d", resp.StatusCode)
	}

	var stack Stack
	if err := json.NewDecoder(resp.Body).Decode(&stack); err != nil {
		return nil, err
	}

	return &stack, nil
}

func (c *Client) GetEndpoint(ctx context.Context, endpointID int) (*Endpoint, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/endpoints/%d", c.baseURL, endpointID), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("portainer api error: status %d", resp.StatusCode)
	}

	var endpoint Endpoint
	if err := json.NewDecoder(resp.Body).Decode(&endpoint); err != nil {
		return nil, err
	}

	return &endpoint, nil
}

// DockerProxy executes a raw request against the underlying Docker engine via Portainer's proxy.
func (c *Client) DockerProxy(ctx context.Context, endpointID int, method, path string, body io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("%s/api/endpoints/%d/docker%s", c.baseURL, endpointID, path)
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-API-Key", c.apiKey)
	return c.http.Do(req)
}
