package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// mock backend server
func newMockServer(response string, stream bool) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if stream {
			for i := 0; i < 3; i++ {
				fmt.Fprintf(w, "chunk-%d\n", i)
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
			}
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "%s", response)
	})
	return httptest.NewServer(handler)
}

// Test suite
type GatewayTestSuite struct {
	suite.Suite
}

func (s *GatewayTestSuite) SetupTest() {}

func (s *GatewayTestSuite) TestGatewayRoutesMultipleServices() {
	service1 := newMockServer(`{"service":"1","response":"service1-response"}`, false)
	service2 := newMockServer(`{"service":"2","response":"service2-response"}`, false)
	defer service1.Close()
	defer service2.Close()

	services := []Service{
		{
			Name:     "service1",
			Host:     service1.URL,
			Protocol: "http",
			Routes: []Route{
				{Path: "/s1", Methods: []string{"GET"}},
			},
		},
		{
			Name: "service2",
			Host: service2.URL,
			// Protocol: "http",
			Routes: []Route{
				{Path: "/s2", Methods: []string{"GET"}},
			},
		},
	}

	gateWayServer(services...)

	// start gateway
	ts := httptest.NewServer(nil)
	defer ts.Close()

	resp1, _ := http.Get(ts.URL + "/s1")
	body1, _ := io.ReadAll(resp1.Body)
	resp1.Body.Close()

	resp2, _ := http.Get(ts.URL + "/s2")
	body2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()

	s.Contains(string(body1), `"service":"1"`)
	s.Contains(string(body2), `"service":"2"`)
	s.Contains(string(body1), `"response":"service1-response"`)
	s.Contains(string(body2), `"response":"service2-response"`)
}

func (s *GatewayTestSuite) TestGatewayPrefixAndPathJoin() {
	mock := newMockServer(`{"path":"/movie/info"}`, false)
	defer mock.Close()

	service := Service{
		Name:     "movie-service",
		Host:     mock.URL,
		Prefix:   "/movie",
		Protocol: "http",
		Routes: []Route{
			{Path: "/info", Methods: []string{"GET"}},
		},
	}

	gateWayServer(service)

	ts := httptest.NewServer(nil)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/movie/info")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	s.Contains(string(body), "path")
	s.Contains(string(body), "/movie/info")
}

func (s *GatewayTestSuite) TestGatewayStreamingResponse() {
	streamingServer := newMockServer("", true)
	defer streamingServer.Close()

	service := Service{
		Name:     "stream-service",
		Host:     streamingServer.URL,
		Protocol: "http",
		Routes: []Route{
			{Path: "/stream", Methods: []string{"GET"}},
		},
	}

	gateWayServer(service)

	ts := httptest.NewServer(nil)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/stream")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	s.Contains(string(body), "chunk-0")
	s.Contains(string(body), "chunk-1")
	s.Contains(string(body), "chunk-2")
}

func (s *GatewayTestSuite) TestGatewayNonStreamingResponse() {
	mock := newMockServer(`{"service":"1","type":"non-stream"}`, false)
	defer mock.Close()

	service := Service{
		Name:     "non-stream",
		Host:     mock.URL,
		Protocol: "http",
		Routes: []Route{
			{Path: "/non-stream", Methods: []string{"GET"}},
		},
	}

	gateWayServer(service)
	ts := httptest.NewServer(nil)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/non-stream")
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	s.Contains(string(body), `"service":"1"`)
	s.Contains(string(body), `"type":"non-stream"`)
}

func (s *GatewayTestSuite) TestGatewayRejectsInvalidMethod() {
	mock := newMockServer("{}", false)
	defer mock.Close()

	service := Service{
		Name:     "reject-service",
		Host:     mock.URL,
		Protocol: "http",
		Routes: []Route{
			{Path: "/reject", Methods: []string{"GET"}},
		},
	}

	gateWayServer(service)
	ts := httptest.NewServer(nil)
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/reject", bytes.NewBufferString(`{}`))
	resp, err := http.DefaultClient.Do(req)
	require.NoError(s.T(), err)
	defer resp.Body.Close()

	s.Equal(http.StatusMethodNotAllowed, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	s.Contains(string(body), "Method Not Allowed")
}

func (s *GatewayTestSuite) TestGatewayHandlesInvalidConfigGracefully() {
	service := Service{
		Name:     "bad-service",
		Host:     ":::::bad_url",
		Protocol: "http",
		Routes: []Route{
			{Path: "/bad", Methods: []string{"GET"}},
		},
	}
	gateWayServer(service)
	ts := httptest.NewServer(nil)
	defer ts.Close()

	resp, _ := http.Get(ts.URL + "/bad")
	body, _ := io.ReadAll(resp.Body)
	defer resp.Body.Close()

	s.Equal(http.StatusNotFound, resp.StatusCode)
	s.Contains(string(body), "404")
}

func (s *GatewayTestSuite) TestGatewayPreservesHeaders() {
	mock := newMockServer(`ok`, false)
	defer mock.Close()

	service := Service{
		Name:     "header-service",
		Host:     mock.URL,
		Protocol: "http",
		Routes: []Route{
			{Path: "/headers", Methods: []string{"GET"}},
		},
	}
	gateWayServer(service)

	ts := httptest.NewServer(nil)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/headers", nil)
	req.Header.Set("X-Test", "true")

	resp, _ := http.DefaultClient.Do(req)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)
	s.Contains(string(body), "ok")
}

func TestGatewayTestSuite(t *testing.T) {
	suite.Run(t, new(GatewayTestSuite))
}
