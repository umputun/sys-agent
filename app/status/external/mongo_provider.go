package external

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"text/template"
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

	now func() time.Time // for testing
}

// Status returns status of mongo, checks if connection established and ping is ok
// request URL looks like mongo:mongodb://172.17.42.1:27017/test?oplogMaxDelta=30m
// oplogMaxDelta is optional, if set, checks if oplog is not too far behind
func (m *MongoProvider) Status(req Request) (*Response, error) {
	st := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), m.TimeOut)
	defer cancel()

	if m.now == nil {
		m.now = time.Now
	}

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

	count, err := m.countQuery(ctx, client, uu)
	if err != nil {
		return nil, fmt.Errorf("mongo count query failed: %s %s: %w", req.Name, req.URL, err)
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
	if count >= 0 {
		result.Body["count"] = count
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
		// the code below doing type assertions.
		// Even if each case is covered/checked we better have recover, just in case
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

// countQuery returns number of documents in the collection matching the filter
// request URL looks like this:	mongo:mongodb://blah:27017/test?db=test&collection=users&count={"status":"active"}
func (m *MongoProvider) countQuery(ctx context.Context, client *mdrv.Client, req *url.URL) (int64, error) {
	countQuery := req.Query().Get("count")
	if countQuery == "" {
		return -1, nil // no count filter requested
	}
	dt := NewDayTemplate(m.now())
	countQuery = dt.Parse(countQuery) // replace date templates with actual dates, i.e. {"date": "[[.YYYYMMDD]]:00:00:00Z"}

	collection := req.Query().Get("collection")
	db := req.Query().Get("db")
	if collection == "" || db == "" {
		return 0, fmt.Errorf("collection and db should be provided for count query")
	}
	filter := bson.M{}
	if err := bson.UnmarshalExtJSON([]byte(countQuery), false, &filter); err != nil {
		return 0, fmt.Errorf("mongo filter can't be parsed: %w", err)
	}
	coll := client.Database(db).Collection(collection)
	count, err := coll.CountDocuments(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("mongo count failed: %w", err)
	}
	return count, nil
}

// DayTemplate used to translate templates with date info
type DayTemplate struct {
	YYYYMMDD  string
	YYYYMMDD1 string // yesterday
	YYYYMMDD2 string // -2 days
	YYYYMMDD3 string // -3 days
	YYYYMMDD4 string // -4 days
	YYYYMMDD5 string // -5 days
	YYYYMMDD6 string // -6 days
	YYYYMMDD7 string // -7 days
	NOW       string // now time, with seconds precision
}

// NewDayTemplate makes day parser for given date
// ts - time to make template for
func NewDayTemplate(ts time.Time) *DayTemplate {
	lts := ts.In(time.UTC)
	d := &DayTemplate{
		YYYYMMDD: fmt.Sprintf(`{"$date":"%04d-%02d-%02dT%02d:%02d:%02dZ"}`,
			lts.Year(), lts.Month(), lts.Day(), lts.Hour(), lts.Minute(), lts.Second()),
		YYYYMMDD1: fmt.Sprintf(`{"$date":"%04d-%02d-%02dT%02d:%02d:%02dZ"}`,
			lts.AddDate(0, 0, -1).Year(), lts.AddDate(0, 0, -1).Month(), lts.AddDate(0, 0, -1).Day(),
			lts.AddDate(0, 0, -1).Hour(), lts.AddDate(0, 0, -1).Minute(), lts.AddDate(0, 0, -1).Second()),
		YYYYMMDD2: fmt.Sprintf(`{"$date":"%04d-%02d-%02dT%02d:%02d:%02dZ"}`,
			lts.AddDate(0, 0, -2).Year(), lts.AddDate(0, 0, -2).Month(), lts.AddDate(0, 0, -2).Day(),
			lts.AddDate(0, 0, -2).Hour(), lts.AddDate(0, 0, -2).Minute(), lts.AddDate(0, 0, -2).Second()),
		YYYYMMDD3: fmt.Sprintf(`{"$date":"%04d-%02d-%02dT%02d:%02d:%02dZ"}`,
			lts.AddDate(0, 0, -3).Year(), lts.AddDate(0, 0, -3).Month(), lts.AddDate(0, 0, -3).Day(),
			lts.AddDate(0, 0, -3).Hour(), lts.AddDate(0, 0, -3).Minute(), lts.AddDate(0, 0, -3).Second()),
		YYYYMMDD4: fmt.Sprintf(`{"$date":"%04d-%02d-%02dT%02d:%02d:%02dZ"}`,
			lts.AddDate(0, 0, -4).Year(), lts.AddDate(0, 0, -4).Month(), lts.AddDate(0, 0, -4).Day(),
			lts.AddDate(0, 0, -4).Hour(), lts.AddDate(0, 0, -4).Minute(), lts.AddDate(0, 0, -4).Second()),
		YYYYMMDD5: fmt.Sprintf(`{"$date":"%04d-%02d-%02dT%02d:%02d:%02dZ"}`,
			lts.AddDate(0, 0, -5).Year(), lts.AddDate(0, 0, -5).Month(), lts.AddDate(0, 0, -5).Day(),
			lts.AddDate(0, 0, -5).Hour(), lts.AddDate(0, 0, -5).Minute(), lts.AddDate(0, 0, -5).Second()),
		NOW: fmt.Sprintf(`{"$date":"%04d-%02d-%02dT%02d:%02d:%02dZ"}`,
			lts.Year(), lts.Month(), lts.Day(), lts.Hour(), lts.Minute(), lts.Second()),
	}

	return d
}

// Parse translate template to final string
func (d DayTemplate) Parse(dayTemplate string) string {
	b1 := bytes.Buffer{}
	tmpl := template.New("ymd").Delims("[[", "]]")
	err := template.Must(tmpl.Parse(dayTemplate)).Execute(&b1, d)
	if err != nil {
		log.Printf("[WARN] failed to parse day from %s", dayTemplate)
	}
	return b1.String()
}
