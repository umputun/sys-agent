package external

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/go-pkgz/mongo/v2"

	mopt "go.mongodb.org/mongo-driver/mongo/options"
)

// MongoProvider is a status provider that uses mongo
type MongoProvider struct {
	TimeOut time.Duration
}

// Status returns status of mongo, checks if connection established and ping is ok
func (m *MongoProvider) Status(req Request) (*Response, error) {
	st := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), m.TimeOut)
	defer cancel()

	client, _, err := mongo.Connect(ctx, mopt.Client().SetAppName("sys-agent").SetConnectTimeout(m.TimeOut), req.URL)
	if err != nil {
		return nil, fmt.Errorf("mongo connect failed: %s %s: %w", req.Name, req.URL, err)
	}
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			log.Printf("[WARN] mongo disconnect failed: %s %s: %v", req.Name, req.URL, err)
		}
	}()
	result := Response{
		Name:         req.Name,
		StatusCode:   200,
		Body:         map[string]interface{}{"status": "ok"},
		ResponseTime: time.Since(st).Milliseconds(),
	}
	return &result, nil
}
