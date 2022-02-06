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
	"go.mongodb.org/mongo-driver/bson/primitive"
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
		if e := client.Disconnect(ctx); e != nil {
			log.Printf("[WARN] mongo disconnect failed: %s %s: %v", req.Name, req.URL, e)
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
	if rs != nil {
		result.Body["rs"] = rs
	}
	return &result, nil
}

// replStatus gets replica set status if mongo configured as replica set
// for standalone mongo returns nil map
func (m *MongoProvider) replStatus(ctx context.Context, client *mdrv.Client, req *url.URL) (*replSet, error) {

	rs := client.Database("admin").RunCommand(ctx, bson.M{"replSetGetStatus": 1})
	if rs.Err() != nil {
		if !strings.Contains(rs.Err().Error(), "NoReplicationEnabled") {
			return nil, fmt.Errorf("mongo replset can't be extracted: %w", rs.Err())
		}
		return nil, nil // standalone mongo
	}

	rr := bson.M{}
	if err := rs.Decode(&rr); err != nil {
		return nil, fmt.Errorf("mongo replset info can't be decoded: %w", err)
	}

	rsInfo, err := m.parseReplStatus(req, rr)
	if err != nil {
		return nil, fmt.Errorf("mongo replset info can't be parsed: %w", err)
	}
	return rsInfo, nil
}

type replSet struct {
	Set          string          `json:"set"`
	Status       string          `json:"status"`
	OptimeStatus string          `json:"optime"`
	Members      []replSetMember `json:"members"`
}

type replSetMember struct {
	Name   string    `json:"name"`
	State  string    `json:"state"`
	Optime time.Time `json:"optime"`
}

// parseReplStatus parses replSet status bson.M and returns replSet struct
// it supports multiple flavors of replSet status returned by various mongo versions
func (m *MongoProvider) parseReplStatus(req *url.URL, data bson.M) (res *replSet, err error) {

	defer func() {
		// the code below doing type assertions. Even if each case is covered/checked we better have recover, just in case
		if r := recover(); r != nil {
			err = fmt.Errorf("failed: %v", r)
		}
	}()

	oplogMaxDelta := time.Minute
	if req.Query().Get("oplogMaxDelta") != "" {
		d, err := time.ParseDuration(req.Query().Get("oplogMaxDelta"))
		if err != nil {
			return nil, fmt.Errorf("can't parse oplogMaxDelta: %s: %w", req.Host, err)
		}
		oplogMaxDelta = d
	}
	members, ok := data["members"].(primitive.A)
	if !ok {
		return nil, fmt.Errorf("mongo replset members can't be extracted: %+v", data["members"])
	}

	replset := &replSet{
		OptimeStatus: "ok",
		Status:       "ok",
	}

	if replset.Set, ok = data["set"].(string); !ok {
		return nil, fmt.Errorf("mongo replset set can't be extracted: %+v", data["set"])
	}

	for _, m := range members {
		member := replSetMember{}
		if member.Name, ok = m.(bson.M)["name"].(string); !ok {
			return nil, fmt.Errorf("mongo replset member name can't be extracted: %+v", m)
		}
		if member.State, ok = m.(bson.M)["stateStr"].(string); !ok {
			return nil, fmt.Errorf("mongo replset member state can't be extracted: %+v", m)
		}

		switch v := m.(bson.M)["optime"].(type) {
		case time.Time:
			member.Optime = v
		case primitive.M:
			ts, ok := v["ts"].(primitive.Timestamp)
			if !ok {
				return nil, fmt.Errorf("mongo replset member optime can't be extracted: %+v", m)
			}
			member.Optime = time.Unix(int64(ts.T), int64(ts.I))
		}
		replset.Members = append(replset.Members, member)
	}
	if len(replset.Members) == 0 {
		return nil, fmt.Errorf("mongo replset is empty")
	}

	primOptime := replset.Members[0].Optime
	for _, m := range replset.Members {
		if m.State != "PRIMARY" && m.State != "SECONDARY" && m.State != "ARBITER" {
			replset.Status = fmt.Sprintf("failed, invalid state %s for %s", m.State, m.Name)
			break
		}
		if m.State == "SECONDARY" && primOptime.Sub(m.Optime) > oplogMaxDelta {
			replset.OptimeStatus = fmt.Sprintf("failed, optime difference for %s is %v", m.Name, primOptime.Sub(m.Optime))
			break
		}
	}

	return replset, nil
}
