package external

import (
	"context"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-pkgz/syncs"
)

//go:generate moq -out provider_mock.go -skip-ensure -fmt goimports . StatusProvider

// Service wraps multiple StatusProvider and multiplex their Status() calls
type Service struct {
	requests    []Request
	concurrency int
	providers   Providers
}

// Providers is a list of StatusProvider
type Providers struct {
	HTTP   StatusProvider
	Mongo  StatusProvider
	Docker StatusProvider
}

// StatusProvider is an interface for getting status from external services
type StatusProvider interface {
	Status(req Request) (*Response, error)
}

// Request is a name and request to external service
type Request struct {
	Name string
	URL  string
}

// Response contains extended service information
type Response struct {
	Name         string                 `json:"name"`
	StatusCode   int                    `json:"status_code"`
	ResponseTime int64                  `json:"response_time"` // milliseconds
	Body         map[string]interface{} `json:"body,omitempty"`
}

// NewService creates new external service supporting multiple providers
// reqs are requests to external services presented as pairs of name and url, i.e. health:http://localhost:8080/health
func NewService(providers Providers, concurrency int, reqs ...string) *Service {
	result := &Service{
		concurrency: concurrency,
		providers:   providers,
	}

	for _, r := range reqs {
		var req Request
		if i := strings.Index(r, ":"); i > 0 {
			req.Name = r[:i]
			req.URL = r[i+1:]

			if len(req.Name) > 0 && len(req.URL) > 0 {
				result.requests = append(result.requests, req)
			}
			log.Printf("[DEBUG] ext_service: %s:%s", req.Name, req.URL)
		}
	}
	return result
}

// Status returns extended service information, runs concurrently
func (s *Service) Status() []Response {
	if len(s.requests) == 0 {
		return nil
	}
	res := make([]Response, 0, len(s.requests))
	wg := syncs.NewSizedGroup(s.concurrency, syncs.Preemptive)
	ch := make(chan Response, len(s.requests))
	for _, req := range s.requests {
		r := req

		wg.Go(func(ctx context.Context) {

			var (
				resp *Response
				err  error
			)

			st := time.Now()
			switch {
			case strings.HasPrefix(r.URL, "http://") || strings.HasPrefix(r.URL, "https://"):
				resp, err = s.providers.HTTP.Status(r)
			case strings.HasPrefix(r.URL, "mongodb://"):
				resp, err = s.providers.Mongo.Status(r)
			case strings.HasPrefix(r.URL, "docker://"):
				resp, err = s.providers.Docker.Status(r)
			default:
				log.Printf("[WARN] unsupported protocol for ext_service, %s %s", r.Name, r.URL)
				ch <- Response{Name: r.Name, StatusCode: http.StatusInternalServerError, ResponseTime: time.Since(st).Milliseconds()}
				return
			}

			if err != nil {
				log.Printf("[WARN] ext_service request failed: %s %s: %v", r.Name, r.URL, err)
				ch <- Response{Name: r.Name, StatusCode: http.StatusInternalServerError, ResponseTime: time.Since(st).Milliseconds()}
				return
			}

			resp.ResponseTime = time.Since(st).Milliseconds()
			ch <- *resp
			log.Printf("[DEBUG] ext_service response: %s:%s %+v", r.Name, r.URL, *resp)
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
