package dockerengine

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Jellman86/HarborWatch/backend/internal/gen"
)

const defaultDockerSocket = "/var/run/docker.sock"

// Client is a lightweight Docker Engine API client over a Unix socket.
type Client struct {
	httpClient *http.Client
	baseURL    *url.URL
}

func NewFromEnv() (*Client, error) {
	socket := os.Getenv("DOCKER_HOST")
	if socket == "" {
		socket = defaultDockerSocket
	}

	if strings.HasPrefix(socket, "unix://") {
		socket = strings.TrimPrefix(socket, "unix://")
	}
	socket = filepath.Clean(socket)

	if _, err := os.Stat(socket); err != nil {
		return nil, fmt.Errorf("docker socket not available at %s: %w", socket, err)
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		},
	}

	base, _ := url.Parse("http://docker")
	return &Client{
		httpClient: &http.Client{Transport: transport, Timeout: 15 * time.Second},
		baseURL:    base,
	}, nil
}

func (c *Client) ListContainers(ctx context.Context) ([]gen.ContainerSummary, error) {
	var raw []containerJSON
	if err := c.getJSON(ctx, "/containers/json?all=1", &raw); err != nil {
		return nil, err
	}

	out := make([]gen.ContainerSummary, 0, len(raw))
	for _, item := range raw {
		out = append(out, gen.ContainerSummary{
			ID:     item.ID,
			Names:  item.Names,
			Image:  item.Image,
			State:  item.State,
			Status: item.Status,
		})
	}
	return out, nil
}

func (c *Client) ListImages(ctx context.Context) ([]gen.ImageSummary, error) {
	var raw []imageJSON
	if err := c.getJSON(ctx, "/images/json", &raw); err != nil {
		return nil, err
	}

	out := make([]gen.ImageSummary, 0, len(raw))
	for _, item := range raw {
		out = append(out, gen.ImageSummary{
			ID:       item.ID,
			RepoTags: item.RepoTags,
			Size:     item.Size,
		})
	}
	return out, nil
}

func (c *Client) OpenEventStream(ctx context.Context) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL.String()+"/events", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("docker events request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("docker events failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	return resp.Body, nil
}

func (c *Client) getJSON(ctx context.Context, path string, into any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL.String()+path, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("docker request failed for %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("docker request %s failed: status=%d body=%s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(into); err != nil {
		return fmt.Errorf("decode docker response %s: %w", path, err)
	}
	return nil
}

type containerJSON struct {
	ID     string   `json:"Id"`
	Names  []string `json:"Names"`
	Image  string   `json:"Image"`
	State  string   `json:"State"`
	Status string   `json:"Status"`
}

type imageJSON struct {
	ID       string   `json:"Id"`
	RepoTags []string `json:"RepoTags"`
	Size     int64    `json:"Size"`
}
