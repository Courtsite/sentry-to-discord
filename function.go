package function

import (
	"bytes"
	"encoding/json"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type SentryWebhook struct {
	ID string `json:"id"`

	Message string `json:"message"`
	URL     string `json:"url"`
	Culprit string `json:"culprit"`
	Level   string `json:"level"`
	Project string `json:"project"`

	Event struct {
		EventID string `json:"event_id"`

		Environment string  `json:"environment"`
		Received    float64 `json:"received"`
		Timestamp   float64 `json:"timestamp"`
	} `json:"event"`
}

type DiscordWebhook struct {
	Content string  `json:"content"`
	Embeds  []Embed `json:"embeds,omitempty"`
}

type Embed struct {
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	Description string  `json:"description"`
	Color       int     `json:"color"`
	Fields      []Field `json:"fields,omitempty"`
}

type Field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline"`
}

func toDiscord(webhook SentryWebhook) DiscordWebhook {
	levelToDecColour := map[string]int{
		"debug":   13620186,
		"info":    2590926,
		"warning": 15828224,
		"error":   16006944,
		"fatal":   13766442,
	}
	colour, ok := levelToDecColour[strings.ToLower(webhook.Level)]
	if !ok {
		// Defaults to #FFFFFF (white)
		colour = 16777215
	}

	sec, dec := math.Modf(webhook.Event.Timestamp)
	timestamp := time.Unix(int64(sec), int64(dec*(1e9)))

	environment := webhook.Event.Environment
	if environment == "" {
		environment = "unknown"
	}

	return DiscordWebhook{
		Embeds: []Embed{
			{
				Title: webhook.Message,
				URL:   webhook.URL,
				Color: colour,
				Fields: []Field{
					Field{
						Name:   "Project",
						Value:  webhook.Project,
						Inline: true,
					},
					Field{
						Name:   "Environment",
						Value:  environment,
						Inline: true,
					},
					Field{
						Name:  "Timestamp",
						Value: timestamp.String(),
					},
					Field{
						Name:  "Culprit",
						Value: webhook.Culprit,
					},
				},
			},
		},
	}
}

func F(w http.ResponseWriter, r *http.Request) {
	// This is commented out because it was quite hard to test.
	// It may be revisited at some point.
	// if resource := r.Header.Get("Sentry-Hook-Resource"); resource != "metric_alert" {
	// 	w.WriteHeader(http.StatusBadRequest)
	// 	w.Write([]byte("invalid request"))
	// 	log.Printf("invalid Sentry-Hook-Resource: %s\n", resource)
	// 	return
	// }

	authToken := os.Getenv("AUTH_TOKEN")
	if authToken == "" {
		log.Fatalln("`AUTH_TOKEN` is not set in the environment")
	}

	if receivedAuthToken := r.URL.Query().Get("auth_token"); receivedAuthToken != authToken {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid request"))
		log.Printf("invalid auth token: %s\n", receivedAuthToken)
		return
	}

	discordWebhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if discordWebhookURL == "" {
		log.Fatalln("`DISCORD_WEBHOOK_URL` is not set in the environment")
	}

	if _, err := url.Parse(discordWebhookURL); err != nil {
		log.Fatalln(err)
	}

	if contentType := r.Header.Get("Content-Type"); r.Method != "POST" || contentType != "application/json" {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid request"))
		log.Printf("invalid method / content-type: %s / %s\n", r.Method, contentType)
		return
	}

	var webhook SentryWebhook
	if err := json.NewDecoder(r.Body).Decode(&webhook); err != nil {
		log.Fatalln(err)
	}

	discordWebhook := toDiscord(webhook)

	payload, err := json.Marshal(discordWebhook)
	if err != nil {
		log.Fatalln(err)
	}

	res, err := http.Post(discordWebhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Fatalln(err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		log.Println("payload", string(payload))
		log.Fatalln("unexpected status code", res.StatusCode)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(discordWebhook)
}
