package external

import (
	"strconv"
	"testing"

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

	s := NewService(Providers{ph, pm, pd, pp, pn}, 4,
		"s1:http://127.0.0.1/ping", "s2:docker:///var/blah", "s3:mongodb://127.0.0.1:27017",
		"s4:program://ls?arg=1", "bad:bad")

	res := s.Status()
	require.Equal(t, 5, len(res))
	assert.Equal(t, 1, len(ph.StatusCalls()))
	assert.Equal(t, Request{Name: "s1", URL: "http://127.0.0.1/ping"}, ph.StatusCalls()[0].Req)

	assert.Equal(t, 1, len(pm.StatusCalls()))
	assert.Equal(t, Request{Name: "s2", URL: "docker:///var/blah"}, pd.StatusCalls()[0].Req)

	assert.Equal(t, 1, len(pd.StatusCalls()))
	assert.Equal(t, Request{Name: "s3", URL: "mongodb://127.0.0.1:27017"}, pm.StatusCalls()[0].Req)

	assert.Equal(t, 1, len(pp.StatusCalls()))
	assert.Equal(t, Request{Name: "s4", URL: "program://ls?arg=1"}, pp.StatusCalls()[0].Req)

	assert.Equal(t, "bad", res[0].Name)
	assert.Equal(t, 500, res[0].StatusCode)

	assert.Equal(t, "docker", res[1].Name)
	assert.Equal(t, 202, res[1].StatusCode)

	assert.Equal(t, "http", res[2].Name)
	assert.Equal(t, 200, res[2].StatusCode)

	assert.Equal(t, "mongo", res[3].Name)
	assert.Equal(t, 201, res[3].StatusCode)

	assert.Equal(t, "program", res[4].Name)
	assert.Equal(t, 203, res[4].StatusCode)
}
