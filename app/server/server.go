package server

import (
	"context"
	"net/http"
	"time"

	"github.com/didip/tollbooth/v7"
	"github.com/didip/tollbooth_chi"
	"github.com/go-chi/chi/v5"
	log "github.com/go-pkgz/lgr"
	"github.com/go-pkgz/rest"

	"github.com/umputun/sys-agent/app/status"
)

//go:generate moq -out status_mock.go -skip-ensure -fmt goimports . Status

// Rest implement http api invoking remote execution for requested tasks
type Rest struct {
	Listen  string
	Version string
	Status  Status
}

// Status is used to get status info of the server
type Status interface {
	Get() (*status.Info, error)
}

// Run starts http server and closes on context cancellation
func (s *Rest) Run(ctx context.Context) error {
	log.Printf("[INFO] start http server on %s", s.Listen)

	httpServer := &http.Server{
		Addr:              s.Listen,
		Handler:           s.router(),
		ReadHeaderTimeout: time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       time.Second,
		ErrorLog:          log.ToStdLogger(log.Default(), "WARN"),
	}

	go func() {
		<-ctx.Done()
		if httpServer != nil {
			if err := httpServer.Close(); err != nil {
				log.Printf("[ERROR] failed to close http server, %v", err)
			}
		}

	}()

	return httpServer.ListenAndServe()
}

func (s *Rest) router() http.Handler {
	router := chi.NewRouter()
	router.Use(rest.Recoverer(log.Default()))
	router.Use(rest.Throttle(100)) // limit total number of the running requests
	router.Use(rest.AppInfo("sys-agent", "umputun", s.Version))
	router.Use(rest.Ping)
	router.Use(tollbooth_chi.LimitHandler(tollbooth.NewLimiter(10, nil)))

	router.Get("/status", func(w http.ResponseWriter, r *http.Request) {

		resp, err := s.Status.Get()
		if err != nil {
			rest.SendErrorJSON(w, r, log.Default(), http.StatusInternalServerError, err, "failed to get status")
			return
		}
		rest.RenderJSON(w, resp)
	})

	return router
}
