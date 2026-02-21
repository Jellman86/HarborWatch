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

	embed := map[string]interface{}{
		"title":       msg.Title,
		"description": msg.Body,
		"color":       color,
		"footer": map[string]string{
			"text": "HarborWatch - " + msg.Source,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}

	if len(msg.Fields) > 0 {
		var fields []map[string]interface{}
		for k, v := range msg.Fields {
			fields = append(fields, map[string]interface{}{
				"name":   k,
				"value":  v,
				"inline": true,
			})
		}
		embed["fields"] = fields
	}

	payload := map[string]interface{}{
		"username":   "HarborWatch",
		"avatar_url": "https://raw.githubusercontent.com/Jellman86/HarborWatch/main/web/public/logo-64.png",
		"embeds":     []map[string]interface{}{embed},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", d.webhookURL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
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
