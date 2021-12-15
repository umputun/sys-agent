package status

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-pkgz/syncs"
)

// ExtServices is a service that retrieves data from external services via HTTP GET calls
type ExtServices struct {
	svcs []struct {
		Name string
		URL  string
	}
	httpTimeout time.Duration
	concurrency int
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

	res := &ExtServices{httpTimeout: httpTimeout, concurrency: concurrency}
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
		len(res.svcs), res.concurrency, res.httpTimeout)
	return res
}

// Status returns extended service information, request timeout is 5 seconds and runs concurrently
func (es *ExtServices) Status() []ExtServiceResp {
	res := make([]ExtServiceResp, 0, len(es.svcs))
	client := http.Client{Timeout: es.httpTimeout}
	wg := syncs.NewSizedGroup(es.concurrency, syncs.Preemptive)
	ch := make(chan ExtServiceResp, len(es.svcs))
	for _, s := range es.svcs {
		s := s
		wg.Go(func(ctx context.Context) {
			st := time.Now()
			res, err := client.Get(s.URL)
			if err != nil {
				log.Printf("[WARN] ext_service request failed: %s %s: %v", s.Name, s.URL, err)
				ch <- ExtServiceResp{Name: s.Name, StatusCode: http.StatusInternalServerError}
				return
			}
			defer res.Body.Close()

			bodyStr, err := io.ReadAll(res.Body)
			if err != nil {
				log.Printf("[WARN] ext_service read failed: %s %s: %v", s.Name, s.URL, err)
				ch <- ExtServiceResp{Name: s.Name, StatusCode: http.StatusInternalServerError}
				return
			}

			var bodyJson map[string]interface{}
			if err := json.Unmarshal(bodyStr, &bodyJson); err != nil {
				bodyJson = map[string]interface{}{"text": string(bodyStr)}
			}
			resp := ExtServiceResp{
				Name:         s.Name,
				StatusCode:   res.StatusCode,
				ResponseTime: time.Since(st).Milliseconds(),
				Body:         bodyJson,
			}
			ch <- resp
			log.Printf("[DEBUG] ext_service reposne: %s:%s %+v", s.Name, s.URL, resp)
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
