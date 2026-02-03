package external

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-pkgz/syncs"
	"github.com/robfig/cron/v3"
)

//go:generate moq -out provider_mock.go -skip-ensure -fmt goimports . StatusProvider

// Service wraps multiple StatusProvider and multiplex their Status() calls
type Service struct {
	requests    []Request
	concurrency int
	providers   Providers

	lastResponses struct {
		cache map[Request]Response
		mu    sync.RWMutex
	}
	nowFn func() time.Time // for testing
}

// Providers is a list of StatusProvider
type Providers struct {
	HTTP        StatusProvider
	Mongo       StatusProvider
	Docker      StatusProvider
	Program     StatusProvider
	Nginx       StatusProvider
	Certificate StatusProvider
	File        StatusProvider
	RMQ         StatusProvider
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
	Name         string         `json:"name"`
	StatusCode   int            `json:"status_code"`
	ResponseTime int64          `json:"response_time"` // milliseconds
	Body         map[string]any `json:"body,omitempty"`
}

// NewService creates new external service supporting multiple providers
// reqs are requests to external services presented as pairs of name and url, i.e. health:http://localhost:8080/health
func NewService(providers Providers, concurrency int, reqs ...string) *Service {
	result := &Service{
		concurrency: concurrency,
		providers:   providers,
		nowFn:       time.Now,
	}
	result.lastResponses.cache = make(map[Request]Response)

	for _, r := range reqs {
		var req Request
		if i := strings.Index(r, ":"); i > 0 {
			req.Name = r[:i]
			req.URL = r[i+1:]

			if req.Name != "" && req.URL != "" {
				result.requests = append(result.requests, req)
			}
			log.Printf("[DEBUG] service: name:%s, url:%s", req.Name, req.URL)
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

		wg.Go(func(context.Context) {
			cronOk, cronErr := s.cronFilter(r.URL)
			if cronErr != nil {
				log.Printf("[WARN] failed to parse cron expression for service %s: %v", r.Name, cronErr)
			}
			if !cronOk && cronErr == nil {
				log.Printf("[DEBUG] skipping service %s, cron expression does not match", r.Name)
				s.lastResponses.mu.RLock()
				if lastResp, ok := s.lastResponses.cache[r]; ok {
					ch <- lastResp // respond with last response if cron expression does not match
				}
				s.lastResponses.mu.RUnlock()
				return
			}

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
			case strings.HasPrefix(r.URL, "program://"):
				resp, err = s.providers.Program.Status(r)
			case strings.HasPrefix(r.URL, "nginx://"):
				resp, err = s.providers.Nginx.Status(r)
			case strings.HasPrefix(r.URL, "cert://"):
				resp, err = s.providers.Certificate.Status(r)
			case strings.HasPrefix(r.URL, "file://"):
				resp, err = s.providers.File.Status(r)
			case strings.HasPrefix(r.URL, "rmq://"):
				resp, err = s.providers.RMQ.Status(r)
			default:
				log.Printf("[WARN] unsupported protocol for service, %s %s", r.Name, r.URL)
				ch <- Response{Name: r.Name, StatusCode: http.StatusInternalServerError, ResponseTime: time.Since(st).Milliseconds()}
				return
			}

			if err != nil {
				log.Printf("[WARN] service request failed: %s %s: %v", r.Name, r.URL, err)
				ch <- Response{Name: r.Name, StatusCode: http.StatusInternalServerError, ResponseTime: time.Since(st).Milliseconds()}
				return
			}

			resp.ResponseTime = time.Since(st).Milliseconds()
			ch <- *resp

			// storing last response to cache, will be used to respond to cron-excluded requests
			s.lastResponses.mu.Lock()
			s.lastResponses.cache[r] = *resp
			s.lastResponses.mu.Unlock()

			log.Printf("[DEBUG] service response: %s:%s %+v", r.Name, r.URL, *resp)
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

// cronFilter checks if the current time matches the cron expression in the URL
// If no cron expression is set, always return true
func (s *Service) cronFilter(reqURL string) (bool, error) {
	parsedURL, err := url.Parse(reqURL)
	if err != nil {
		return false, fmt.Errorf("failed to parse URL: %w", err)
	}

	cronExpr := parsedURL.Query().Get("cron")
	if cronExpr == "" {
		return true, nil // no cron expression is set, return true
	}
	cronExpr = strings.TrimSpace(cronExpr)
	cronExpr = strings.ReplaceAll(cronExpr, "_", " ")

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(cronExpr)
	if err != nil {
		return false, err
	}

	now := s.nowFn()
	nextTime := schedule.Next(now)
	diff := nextTime.Sub(now)
	return now.Before(nextTime) && diff <= time.Minute, nil
}
