package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

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

type GatewayTestSuite struct {
	suite.Suite
}

func (s *GatewayTestSuite) SetupTest() {
	os.Setenv("debug", "test")
}

func (s *GatewayTestSuite) TearDownTest() {
}

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
			Routes: []Route{
				{Path: "/s2", Methods: []string{"GET"}},
			},
		},
	}

	router := gateWayServer(services...)

	ts := httptest.NewServer(router)
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

	router := gateWayServer(service)

	ts := httptest.NewServer(router)
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

	router := gateWayServer(service)

	ts := httptest.NewServer(router)
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

	router := gateWayServer(service)

	ts := httptest.NewServer(router)
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

	router := gateWayServer(service)

	ts := httptest.NewServer(router)
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
	router := gateWayServer(service)
	ts := httptest.NewServer(router)
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
	router := gateWayServer(service)
	ts := httptest.NewServer(router)
	defer ts.Close()

	req, _ := http.NewRequest("GET", ts.URL+"/headers", nil)
	req.Header.Set("X-Test", "true")

	resp, _ := http.DefaultClient.Do(req)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	s.Equal(http.StatusOK, resp.StatusCode)
	s.Contains(string(body), "ok")
}

func (s *GatewayTestSuite) TestRateLimitRetryAfter() {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok": true}`))
	}))
	defer mock.Close()

	service := Service{
		Name: "rate-limit-service",
		Host: mock.URL,
		Routes: []Route{
			{Path: "/limited", Methods: []string{"GET"}},
		},
		RateLimit: RateLimit{
			RequestsPerMinute: 2,
		},
	}

	router := gateWayServer(service)

	ts := httptest.NewServer(router)
	defer ts.Close()

	client := &http.Client{}

	for i := 0; i < 2; i++ {
		resp, err := client.Get(ts.URL + "/limited")
		require.NoError(s.T(), err)
		require.Equal(s.T(), http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	resp, err := client.Get(ts.URL + "/limited")
	require.NoError(s.T(), err)
	defer resp.Body.Close()

	s.Equal(http.StatusTooManyRequests, resp.StatusCode)

	retryAfter := resp.Header.Get("Retry-After")
	s.NotEmpty(retryAfter, "Retry-After header must be set")
}

func (s *GatewayTestSuite) TestRateLimitPerService() {
	service1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"service":"1"}`))
	}))
	defer service1.Close()

	service2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"service":"2"}`))
	}))
	defer service2.Close()

	svc1 := Service{
		Name: "svc1",
		Host: service1.URL,
		Routes: []Route{
			{Path: "/s1", Methods: []string{"GET"}},
		},
		RateLimit: RateLimit{RequestsPerMinute: 2},
	}

	svc2 := Service{
		Name: "svc2",
		Host: service2.URL,
		Routes: []Route{
			{Path: "/s2", Methods: []string{"GET"}},
		},
		RateLimit: RateLimit{RequestsPerMinute: 3},
	}

	router := gateWayServer(svc1, svc2)
	ts := httptest.NewServer(router)
	defer ts.Close()

	client := &http.Client{}

	for i := 0; i < 2; i++ {
		resp, err := client.Get(ts.URL + "/s1")
		require.NoError(s.T(), err)
		require.Equal(s.T(), http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	resp, err := client.Get(ts.URL + "/s1")
	require.NoError(s.T(), err)
	defer resp.Body.Close()
	s.Equal(http.StatusTooManyRequests, resp.StatusCode)
	s.NotEmpty(resp.Header.Get("Retry-After"))

	for i := 0; i < 3; i++ {
		resp, err := client.Get(ts.URL + "/s2")
		require.NoError(s.T(), err)
		s.Equal(http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}

	resp, err = client.Get(ts.URL + "/s2")
	require.NoError(s.T(), err)
	defer resp.Body.Close()
	s.Equal(http.StatusTooManyRequests, resp.StatusCode)
	s.NotEmpty(resp.Header.Get("Retry-After"))
}

func TestGatewayTestSuite(t *testing.T) {
	suite.Run(t, new(GatewayTestSuite))
}
