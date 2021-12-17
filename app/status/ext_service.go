package status

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-pkgz/mongo/v2"
	"github.com/go-pkgz/syncs"
	mopt "go.mongodb.org/mongo-driver/mongo/options"
)

// ExtServices is a service that retrieves data from external services via HTTP GET calls
type ExtServices struct {
	svcs        []ExtServiceReq
	timeout     time.Duration
	concurrency int
}

// ExtServiceReq is a name and request to external service
type ExtServiceReq struct {
	Name string
	URL  string
}

// ExtServiceResp contains extended service information
type ExtServiceResp struct {
	Name         string                 `json:"name"`
	StatusCode   int                    `json:"status_code"`
	ResponseTime int64                  `json:"response_time"` // milliseconds
	Body         map[string]interface{} `json:"body,omitempty"`
}

// NewExtServices returns a new instance of ExtServices for a list of name:url pairs
func NewExtServices(httpTimeout time.Duration, concurrency int, ss ...string) *ExtServices {
	var svc struct {
		Name string
		URL  string
	}

	res := &ExtServices{timeout: httpTimeout, concurrency: concurrency}
	for _, s := range ss {
		if len(s) > 0 {

			if i := strings.Index(s, ":"); i > 0 {
				svc.Name = s[:i]
				svc.URL = s[i+1:]

				if len(svc.Name) > 0 && len(svc.URL) > 0 {
					res.svcs = append(res.svcs, svc)
				}
				log.Printf("[DEBUG] ext_service: %s:%s", svc.Name, svc.URL)
			}
		}
	}
	log.Printf("[INFO] external services checker created for %d services, concurrency:%d, timeout:%v",
		len(res.svcs), res.concurrency, res.timeout)
	return res
}

// Status returns extended service information, request timeout is 5 seconds and runs concurrently
func (es *ExtServices) Status() []ExtServiceResp {
	res := make([]ExtServiceResp, 0, len(es.svcs))
	wg := syncs.NewSizedGroup(es.concurrency, syncs.Preemptive)
	ch := make(chan ExtServiceResp, len(es.svcs))
	for _, s := range es.svcs {
		s := s

		wg.Go(func(ctx context.Context) {

			var (
				resp *ExtServiceResp
				err  error
			)

			st := time.Now()
			switch {
			case strings.HasPrefix(s.URL, "http://") || strings.HasPrefix(s.URL, "https://"):
				resp, err = es.httpStatus(s)
			case strings.HasPrefix(s.URL, "mongodb://"):
				resp, err = es.mongoStatus(s)
			default:
				log.Printf("[WARN] unsupported protocol for ext_service, %s %s", s.Name, s.URL)
				ch <- ExtServiceResp{Name: s.Name, StatusCode: http.StatusInternalServerError, ResponseTime: time.Since(st).Milliseconds()}
				return
			}

			if err != nil {
				log.Printf("[WARN] ext_service request failed: %s %s: %v", s.Name, s.URL, err)
				ch <- ExtServiceResp{Name: s.Name, StatusCode: http.StatusInternalServerError, ResponseTime: time.Since(st).Milliseconds()}
				return
			}

			resp.ResponseTime = time.Since(st).Milliseconds()
			ch <- *resp
			log.Printf("[DEBUG] ext_service response: %s:%s %+v", s.Name, s.URL, *resp)
		})
	}
	wg.Wait()
	close(ch)

	for r := range ch {
		res = append(res, r)
	}
	sort.Slice(res, func(i, j int) bool { return res[i].Name < res[j].Name })
	return res
}

func (es *ExtServices) httpStatus(req ExtServiceReq) (*ExtServiceResp, error) {
	client := http.Client{Timeout: es.timeout}
	resp, err := client.Get(req.URL)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %s %s: %w", req.Name, req.URL, err)
	}
	defer resp.Body.Close()

	bodyStr, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("http read failed: %s %s: %w", req.Name, req.URL, err)
	}

	var bodyJSON map[string]interface{}
	if err := json.Unmarshal(bodyStr, &bodyJSON); err != nil {
		bodyJSON = map[string]interface{}{"text": string(bodyStr)}
	}
	result := ExtServiceResp{
		Name:       req.Name,
		StatusCode: resp.StatusCode,
		Body:       bodyJSON,
	}
	return &result, nil
}

func (es *ExtServices) mongoStatus(req ExtServiceReq) (*ExtServiceResp, error) {
	ctx, cancel := context.WithTimeout(context.Background(), es.timeout)
	defer cancel()

	client, _, err := mongo.Connect(ctx, mopt.Client().SetAppName("sys-agent").SetConnectTimeout(es.timeout), req.URL)
	if err != nil {
		return nil, fmt.Errorf("mongo connect failed: %s %s: %w", req.Name, req.URL, err)
	}
	defer func() {
		if err := client.Disconnect(ctx); err != nil {
			log.Printf("[WARN] mongo disconnect failed: %s %s: %v", req.Name, req.URL, err)
		}
	}()
	result := ExtServiceResp{
		Name:       req.Name,
		StatusCode: 200,
		Body:       map[string]interface{}{"status": "ok"},
	}
	return &result, nil
}
