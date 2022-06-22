package external

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// CertificateProvider is a status provider that check SSL certificate
type CertificateProvider struct {
	TimeOut time.Duration
}

// Status url looks like: cert://example.com. It will try to get SSL certificate and check if it is valid and not going to expire soon
func (c *CertificateProvider) Status(req Request) (*Response, error) {
	st := time.Now()
	addr := strings.TrimPrefix(req.URL, "cert://") + ":443"
	conn, err := tls.Dial("tcp", addr, &tls.Config{}) //nolint:gosec // we don't care about cert version
	if err != nil {
		return nil, errors.Wrapf(err, "failed to connect to %s", addr)
	}
	if err = conn.Handshake(); err != nil {
		return nil, errors.Wrapf(err, "failed to handshake with %s", addr)
	}
	defer conn.Close() // nolint

	certs := conn.ConnectionState().PeerCertificates
	earlierCert := time.Date(2150, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, cert := range certs {
		if cert.NotAfter.Before(earlierCert) {
			earlierCert = cert.NotAfter
		}
	}

	daysLeft := int(time.Until(earlierCert).Hours() / 24)
	body := map[string]interface{}{
		"expire":    earlierCert.Format(time.RFC3339),
		"days_left": daysLeft,
		"host":      strings.Replace(req.URL, "cert://", "https://", 1),
		"status":    "ok",
	}
	if daysLeft < 5 {
		body["status"] = fmt.Sprintf("expiring soon, in %d days", daysLeft)
	}
	if earlierCert.Before(time.Now()) {
		body["status"] = "expired"
	}

	result := Response{
		Name:         req.Name,
		StatusCode:   200,
		Body:         body,
		ResponseTime: time.Since(st).Milliseconds(),
	}
	return &result, nil
}
