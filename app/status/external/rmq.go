package external

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// RMQProvider is a status provider that uses RabbitMQ management API
type RMQProvider struct {
	TimeOut  time.Duration
	lastMsgs int
}

// Status returns the status for a given queue via RabbitMQ management API
// Status url looks like: rmq://user:passwd@example.com:12345/queues/vhost/queue_name. It will try https first and if it fails http
func (h *RMQProvider) Status(req Request) (*Response, error) {

	rec := struct {
		Name               string `json:"name"`
		BackingQueueStatus struct {
			AvgIngressRate float64 `json:"avg_ingress_rate"`
			AvgEgressRate  float64 `json:"avg_egress_rate"`
		} `json:"backing_queue_status"`
		Consumers        int `json:"consumers"`
		Messages         int `json:"messages"`
		MessagesReady    int `json:"messages_ready"`
		MessagesUnack    int `json:"messages_unacknowledged"`
		MessagesReadyRam int `json:"messages_ready_ram"`
		MessagesDetails  struct {
			Rate float64 `json:"rate"`
		} `json:"messages_details"`
		MessageStats struct {
			Publish        int `json:"publish"`
			PublishDetails struct {
				Rate float64 `json:"rate"`
			} `json:"publish_details"`
		} `json:"message_stats"`
		State string `json:"state"`
		Vhost string `json:"vhost"`
	}{}

	st := time.Now()
	client := http.Client{Timeout: h.TimeOut}
	u := strings.Replace(req.URL, "rmq://", "https://", 1)
	resp, err := client.Get(u)
	if err != nil {
		u = strings.Replace(req.URL, "rmq://", "http://", 1)
		resp, err = client.Get(u)
		if err != nil {
			return nil, fmt.Errorf("both https and http failed for %s: %w", req.URL, err)
		}
	}
	defer resp.Body.Close() // nolint
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get RabbitMQ response for %s: %s", req.URL, resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&rec); err != nil {
		return nil, fmt.Errorf("failed to parse RabbitMQ response for %s: %w", req.URL, err)
	}

	body := make(map[string]interface{})
	body["name"] = rec.Name
	body["consumers"] = rec.Consumers
	body["messages"] = rec.Messages
	body["messages_ready"] = rec.MessagesReady
	body["messages_unacknowledged"] = rec.MessagesUnack
	body["messages_ready_ram"] = rec.MessagesReadyRam
	body["messages_rate"] = rec.MessagesDetails.Rate
	body["avg_ingress_rate"] = rec.BackingQueueStatus.AvgIngressRate
	body["avg_egress_rate"] = rec.BackingQueueStatus.AvgEgressRate
	body["publish"] = rec.MessageStats.Publish
	body["publish_rate"] = rec.MessageStats.PublishDetails.Rate
	body["state"] = rec.State
	body["messages_delta"] = rec.Messages - h.lastMsgs
	body["vhost"] = rec.Vhost

	h.lastMsgs = rec.Messages

	result := &Response{Name: req.Name}
	result.StatusCode = resp.StatusCode
	result.ResponseTime = time.Since(st).Milliseconds()
	result.Body = body
	return result, nil
}
