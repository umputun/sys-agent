package external

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewService(t *testing.T) {
	tbl := []struct {
		extPairs []string
		requests []Request
	}{
		{
			[]string{"1:1", "2:2", "3:3"},
			[]Request{{Name: "1", URL: "1"}, {Name: "2", URL: "2"}, {Name: "3", URL: "3"}},
		},
		{
			[]string{"s1:http://127.0.0.1/ping", "s2:docker:///var/blah", "s3:mongodb://127.0.0.1:27017"},
			[]Request{{Name: "s1", URL: "http://127.0.0.1/ping"}, {Name: "s2", URL: "docker:///var/blah"},
				{Name: "s3", URL: "mongodb://127.0.0.1:27017"}},
		},
		{
			[]string{"1:1", "2:2", "3"},
			[]Request{{Name: "1", URL: "1"}, {Name: "2", URL: "2"}},
		},
	}

	for i, tt := range tbl {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			s := NewService(Providers{}, 4, tt.extPairs...)
			require.Equal(t, tt.requests, s.requests)
		})
	}
}

func TestService_Status(t *testing.T) {
	ph := &StatusProviderMock{StatusFunc: func(r Request) (*Response, error) {
		return &Response{StatusCode: 200, Name: "http"}, nil
	}}
	pm := &StatusProviderMock{StatusFunc: func(r Request) (*Response, error) {
		return &Response{StatusCode: 201, Name: "mongo"}, nil
	}}
	pd := &StatusProviderMock{StatusFunc: func(r Request) (*Response, error) {
		return &Response{StatusCode: 202, Name: "docker"}, nil
	}}
	pp := &StatusProviderMock{StatusFunc: func(r Request) (*Response, error) {
		return &Response{StatusCode: 203, Name: "program"}, nil
	}}
	pn := &StatusProviderMock{StatusFunc: func(r Request) (*Response, error) {
		return &Response{StatusCode: 203, Name: "nginx"}, nil
	}}
	pc := &StatusProviderMock{StatusFunc: func(r Request) (*Response, error) {
		return &Response{StatusCode: 204, Name: "cert"}, nil
	}}
	pf := &StatusProviderMock{StatusFunc: func(r Request) (*Response, error) {
		return &Response{StatusCode: 205, Name: "file"}, nil
	}}
	pr := &StatusProviderMock{StatusFunc: func(r Request) (*Response, error) {
		return &Response{StatusCode: 206, Name: "rmq"}, nil
	}}
	s := NewService(Providers{ph, pm, pd, pp, pn, pc, pf, pr}, 4,
		"s1:http://127.0.0.1/ping", "s2:docker:///var/blah", "s3:mongodb://127.0.0.1:27017",
		"s4:program://ls?arg=1", "s5:cert://umputun.com", "s6:file://blah.txt", "s7:rmq://127.0.0.1:5672", "bad:bad")

	res := s.Status()
	require.Len(t, res, 8)
	assert.Len(t, ph.StatusCalls(), 1)
	assert.Equal(t, Request{Name: "s1", URL: "http://127.0.0.1/ping"}, ph.StatusCalls()[0].Req)

	assert.Len(t, pm.StatusCalls(), 1)
	assert.Equal(t, Request{Name: "s2", URL: "docker:///var/blah"}, pd.StatusCalls()[0].Req)

	assert.Len(t, pd.StatusCalls(), 1)
	assert.Equal(t, Request{Name: "s3", URL: "mongodb://127.0.0.1:27017"}, pm.StatusCalls()[0].Req)

	assert.Len(t, pp.StatusCalls(), 1)
	assert.Equal(t, Request{Name: "s4", URL: "program://ls?arg=1"}, pp.StatusCalls()[0].Req)

	assert.Equal(t, "bad", res[0].Name)
	assert.Equal(t, 500, res[0].StatusCode)

	assert.Equal(t, "cert", res[1].Name)
	assert.Equal(t, 204, res[1].StatusCode)

	assert.Equal(t, "docker", res[2].Name)
	assert.Equal(t, 202, res[2].StatusCode)

	assert.Equal(t, "file", res[3].Name)
	assert.Equal(t, 205, res[3].StatusCode)

	assert.Equal(t, "http", res[4].Name)
	assert.Equal(t, 200, res[4].StatusCode)

	assert.Equal(t, "mongo", res[5].Name)
	assert.Equal(t, 201, res[5].StatusCode)

	assert.Equal(t, "program", res[6].Name)
	assert.Equal(t, 203, res[6].StatusCode)

	assert.Equal(t, "rmq", res[7].Name)
	assert.Equal(t, 206, res[7].StatusCode)
}

func TestService_StatusWithCron(t *testing.T) {
	mockHTTP := &StatusProviderMock{
		StatusFunc: func(r Request) (*Response, error) {
			return &Response{StatusCode: 200, Name: r.Name, Body: map[string]any{"status": "ok"}}, nil
		},
	}

	fixedTime := time.Date(2023, 1, 1, 11, 59, 59, 0, time.UTC)
	s := NewService(Providers{HTTP: mockHTTP}, 4,
		"s1:http://example.com?cron=0 12 * * *",  // matches at 12:00
		"s2:http://example.org?cron=*/5_*_*_*_*", // matches every 5 minutes
		"s4:http://example.edu")                  // no cron, should always run

	t.Run("initial run at 12:00", func(t *testing.T) {
		s.nowFn = func() time.Time { return fixedTime }
		s.lastResponses.cache = make(map[Request]Response)
		res := s.Status()
		require.Len(t, res, 3)
		assert.Contains(t, []string{"s1", "s2", "s4"}, res[0].Name)
		assert.Contains(t, []string{"s1", "s2", "s4"}, res[1].Name)
		assert.Contains(t, []string{"s1", "s2", "s4"}, res[2].Name)
	})

	t.Run("run at 12:01, no cache", func(t *testing.T) {
		s.nowFn = func() time.Time { return fixedTime.Add(1 * time.Minute) }
		s.lastResponses.cache = make(map[Request]Response)
		res := s.Status()
		require.Len(t, res, 1)
		assert.Contains(t, []string{"s4"}, res[0].Name)
	})

	t.Run("run at 12:01, with cache", func(t *testing.T) {
		s.nowFn = func() time.Time { return fixedTime }
		s.lastResponses.cache = make(map[Request]Response)
		res := s.Status()
		require.Len(t, res, 3)

		s.nowFn = func() time.Time { return fixedTime.Add(1 * time.Minute) }
		res = s.Status()
		require.Len(t, res, 3)
	})

	t.Run("run at 12:05", func(t *testing.T) {
		s.nowFn = func() time.Time { return fixedTime.Add(5 * time.Minute) }
		s.lastResponses.cache = make(map[Request]Response)
		res := s.Status()
		require.Len(t, res, 2)
		assert.Contains(t, []string{"s2", "s4"}, res[0].Name)
		assert.Contains(t, []string{"s2", "s4"}, res[1].Name)
	})

	t.Run("check caching for skipped checks", func(t *testing.T) {
		// run at 12:00 to populate cache
		s.nowFn = func() time.Time { return fixedTime }
		s.lastResponses.cache = make(map[Request]Response)
		res := s.Status()
		require.Len(t, res, 3)

		// modify mock to return different response
		mockHTTP.StatusFunc = func(r Request) (*Response, error) {
			return &Response{StatusCode: 200, Name: r.Name, Body: map[string]any{"status": "changed"}}, nil
		}

		// run at 12:01
		s.nowFn = func() time.Time { return fixedTime.Add(1 * time.Minute) }
		res = s.Status()

		require.Len(t, res, 3)
		for _, r := range res {
			switch r.Name {
			case "s1":
				assert.Equal(t, "ok", r.Body["status"], "s1 should return cached response")
			case "s2":
				assert.Equal(t, "ok", r.Body["status"], "s2 services should return cached response")
			case "s4":
				assert.Equal(t, "changed", r.Body["status"], "s4 should return new response")
			}
		}
	})
}

func TestService_cronFilter(t *testing.T) {
	s := &Service{}

	t.Run("valid cron expression just after current time", func(t *testing.T) {
		s.nowFn = func() time.Time { return time.Date(2023, 1, 1, 11, 59, 59, 0, time.UTC) }
		ok, err := s.cronFilter("http://example.com?cron=0 12 * * *")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("valid cron expression within the minute", func(t *testing.T) {
		s.nowFn = func() time.Time { return time.Date(2023, 1, 1, 11, 59, 0, 0, time.UTC) }
		ok, err := s.cronFilter("http://example.com?cron=0 12 * * *")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("invalid cron expression", func(t *testing.T) {
		ok, err := s.cronFilter("http://example.com?cron=invalid")
		require.Error(t, err)
		assert.False(t, ok)
	})

	t.Run("no cron expression", func(t *testing.T) {
		ok, err := s.cronFilter("http://example.com")
		require.NoError(t, err)
		assert.True(t, ok)
	})

	t.Run("cron expression not matching current time", func(t *testing.T) {
		s.nowFn = func() time.Time { return time.Date(2023, 1, 1, 12, 1, 0, 0, time.UTC) }
		ok, err := s.cronFilter("http://example.com?cron=0 13 * * *")
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("cron expression just after matching time", func(t *testing.T) {
		s.nowFn = func() time.Time { return time.Date(2023, 1, 1, 12, 0, 1, 999999999, time.UTC) }
		ok, err := s.cronFilter("http://example.com?cron=0 12 * * *")
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("cron expression long after matching time", func(t *testing.T) {
		s.nowFn = func() time.Time { return time.Date(2023, 1, 1, 12, 1, 0, 0, time.UTC) }
		ok, err := s.cronFilter("http://example.com?cron=0 16 * * *")
		require.NoError(t, err)
		assert.False(t, ok)
	})
}
