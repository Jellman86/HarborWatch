package portainer

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Stack represents a Portainer stack.
type Stack struct {
	ID          int    `json:"Id"`
	Name        string `json:"Name"`
	Type        int    `json:"Type"`
	EndpointID  int    `json:"EndpointId"`
	SwarmID     string `json:"SwarmId"`
	EntryPoint  string `json:"EntryPoint"`
	Status      int    `json:"Status"`
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
