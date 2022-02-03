package external

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"github.com/go-pkgz/mongo/v2"
	"go.mongodb.org/mongo-driver/bson"
	mdrv "go.mongodb.org/mongo-driver/mongo"

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

	uu, err := url.Parse(req.URL)
	if err != nil {
		return nil, fmt.Errorf("mongo url parse failed: %s %s: %w", req.Name, req.URL, err)
	}

	rs, err := m.replStatus(ctx, client, uu)
	if err != nil {
		return nil, fmt.Errorf("mongo repl status failed: %s %s: %w", req.Name, req.URL, err)
	}

	result := Response{
		Name:         req.Name,
		StatusCode:   200,
		Body:         map[string]interface{}{"status": "ok"},
		ResponseTime: time.Since(st).Milliseconds(),
	}
	if rs["info"] != nil {
		result.Body["rs"] = rs
	}
	return &result, nil
}

func (m *MongoProvider) replStatus(ctx context.Context, client *mdrv.Client, req *url.URL) (bson.M, error) {

	oplogMaxDelta := time.Minute
	if req.Query().Get("oplogMaxDelta") != "" {
		d, err := time.ParseDuration(req.Query().Get("oplogMaxDelta"))
		if err != nil {
			return nil, fmt.Errorf("can't parse oplogMaxDelta: %s: %w", req.Host, err)
		}
		oplogMaxDelta = d
	}

	rs := client.Database("admin").RunCommand(ctx, bson.D{{"replSetGetStatus", 1}})
	if rs.Err() != nil {
		if !strings.Contains(rs.Err().Error(), "NoReplicationEnabled") {
			return nil, fmt.Errorf("mongo replset can't be extracted: %w", rs.Err())
		}
		return nil, nil
	}

	var replset struct {
		Members []struct {
			Name     string `bson:"name"`
			StateStr string `bson:"stateStr"`
			Optime   struct {
				TS time.Time `bson:"ts"`
			} `bson:"optime"`
		} `bson:"members"`
	}
	if err := rs.Decode(&replset); err != nil {
		return nil, fmt.Errorf("mongo replset can't be decoded: %w", err)
	}
	if len(replset.Members) == 0 {
		return nil, fmt.Errorf("mongo replset is empty")
	}

	primOptime := replset.Members[0].Optime.TS
	status, optime := true, true
	for _, m := range replset.Members {
		if m.StateStr != "PRIMARY" && m.StateStr != "SECONDARY" && m.StateStr != "ARBITER" {
			status = false
			break
		}
		if m.StateStr == "SECONDARY" && primOptime.Sub(m.Optime.TS) > oplogMaxDelta {
			optime = false
			break
		}
	}

	return bson.M{"info": replset, "status": status, "optime": optime}, nil
}
