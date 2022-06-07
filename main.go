package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

type SentryWebhook struct {
	Data struct {
		Error struct {
			EventID string `json:"event_id"`

			Title       string  `json:"title"`
			Culprit     string  `json:"culprit"`
			Level       string  `json:"level"`
			Environment string  `json:"environment"`
			Message     string  `json:"message"`
			Release     string  `json:"release"`
			Timestamp   float64 `json:"timestamp"`
			WebURL      string  `json:"web_url"`
			URL         string  `json:"url"`
		} `json:"error"`
	} `json:"data"`
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
	event := webhook.Data.Error
	colour, ok := levelToDecColour[strings.ToLower(event.Level)]
	if !ok {
		// Defaults to #FFFFFF (white)
		colour = 16777215
	}

	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		panic(err)
	}

	sec, dec := math.Modf(event.Timestamp)
	timestamp := time.Unix(int64(sec), int64(dec*(1e9))).In(loc)

	environment := event.Environment
	if environment == "" {
		environment = "unknown"
	}

	title := strings.TrimSpace(event.Title)
	if title == "" {
		title = strings.TrimSpace(event.Message)
	}

	project := strings.Split(event.URL, "/")[7]

	fields := []Field{
		{
			Name:   "Project",
			Value:  project,
			Inline: true,
		},
		{
			Name:   "Release",
			Value:  event.Release,
			Inline: true,
		},
		{
			Name:   "Environment",
			Value:  environment,
			Inline: true,
		},
		{
			Name:   "Level",
			Value:  event.Level,
			Inline: true,
		},
		{
			Name:  "Culprit",
			Value: event.Culprit,
		},
		{
			Name:  "Timestamp",
			Value: timestamp.String(),
		},
	}

	notBlankFields := []Field{}
	for i := range fields {
		if fields[i].Value != "" {
			notBlankFields = append(notBlankFields, fields[i])
		}
	}

	return DiscordWebhook{
		Embeds: []Embed{
			{
				Title:  title,
				URL:    event.WebURL,
				Color:  colour,
				Fields: notBlankFields,
			},
		},
	}
}

func verifySignature(body []byte, signature []byte, clientSecret []byte) bool {
	h := hmac.New(sha256.New, clientSecret)
	h.Write(body)
	calculated := h.Sum(nil)
	return hmac.Equal(calculated, signature)
}

func verifyTime(timestamp string) (bool, error) {
	time64, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false, err
	}

	if (time.Now().Unix() - time64) > 30 {
		return false, errors.New("Timestamp is too far in the past")
	}

	return true, nil
}

// Handler is your Lambda function handler
// It uses Amazon API Gateway request/responses provided by the aws-lambda-go/events package,
// However you could use other event sources (S3, Kinesis etc), or JSON-decoded primitive types such as 'string'.
func Handler(r events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// stdout and stderr are sent to AWS CloudWatch Logs
	log.Printf("Processing Lambda request %v\n", r.RequestContext)
	log.Printf("Headers: %v\n", r.Headers)

	clientSecret := os.Getenv("CLIENT_SECRET")
	if clientSecret == "" {
		log.Fatalln("`CLIENT_SECRET` is not set in the environment")
	}

	discordWebhookURL := os.Getenv("DISCORD_WEBHOOK_URL")
	if discordWebhookURL == "" {
		log.Fatalln("`DISCORD_WEBHOOK_URL` is not set in the environment")
	}

	if _, err := url.Parse(discordWebhookURL); err != nil {
		log.Fatalln(err)
	}

	if contentType := r.Headers["Content-Type"]; r.HTTPMethod != "POST" || contentType != "application/json" {
		log.Printf("invalid method / content-type: %s / %s\n", r.HTTPMethod, contentType)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
		}, nil
	}

	data := []byte(r.Body)
	log.Printf("raw data received: %s", data)

	messageMAC := r.Headers["Sentry-Hook-Signature"]
	messageMACBuf, _ := hex.DecodeString(messageMAC)
	if !verifySignature(data, messageMACBuf, []byte(clientSecret)) {
		log.Printf("invalid signature\n")
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusUnauthorized,
		}, nil
	}

	if _, err := verifyTime(r.Headers["Sentry-Hook-Timestamp"]); err != nil {
		log.Printf("invalid timestamp: %v\n", err)
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusUnauthorized,
		}, nil
	}

	var webhook SentryWebhook
	if err := json.Unmarshal(data, &webhook); err != nil {
		log.Fatalln(err)
	}

	discordWebhook := toDiscord(webhook)

	payload, err := json.Marshal(discordWebhook)
	if err != nil {
		log.Fatalln(err)
	}

	log.Printf("payload: %s\n", payload)

	res, err := http.Post(discordWebhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Fatalln(err)
	}
	defer res.Body.Close()

	log.Printf("status: %d\n", res.StatusCode)

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		log.Println("payload", string(payload))
		log.Fatalln("unexpected status code", res.StatusCode)

		return events.APIGatewayProxyResponse{
			StatusCode: res.StatusCode,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 204,
	}, nil
}

func main() {
	log.Printf("Start lambda")
	lambda.Start(Handler)
}
