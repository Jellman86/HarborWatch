package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type discordDispatcher struct {
	webhookURL string
	client     *http.Client
}

func NewDiscordDispatcher(url string) Dispatcher {
	return &discordDispatcher{
		webhookURL: url,
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (d *discordDispatcher) Name() string { return "discord" }

func (d *discordDispatcher) Send(ctx context.Context, msg Message) error {
	if d.webhookURL == "" {
		return nil
	}

	color := 0x3498db // Blue (Info)
	if msg.Level == LevelWarning {
		color = 0xf1c40f // Yellow
	} else if msg.Level == LevelCritical {
		color = 0xe74c3c // Red
	}

	payload := map[string]interface{}{
		"embeds": []map[string]interface{}{
			{
				"title":       msg.Title,
				"description": msg.Body,
				"color":       color,
				"footer": map[string]string{
					"text": "HarborWatch - " + msg.Source,
				},
				"timestamp": time.Now().Format(time.RFC3339),
			},
		},
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, "POST", d.webhookURL, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord API returned status %d", resp.StatusCode)
	}

	return nil
}
