package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// helper to create mock downstream services
func mockService(t *testing.T, response string, status int) *httptest.Server {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// setup gateway with provided services

type mockRateLimiter struct{}

func (m *mockRateLimiter) Middleware(_ string, _ float64) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	}
}
func setupGateway(t *testing.T, services map[string]*Service) *Gateway {
	logger := NewLogger()
	gw := NewGateway(logger)
	gw.rateLimiter = NewRateLimiter()
	gw.atomicRoutes.Store(services)
	return gw
}
func TestGateway_SingleServiceRoute(t *testing.T) {
	mock := mockService(t, `{"message":"user service ok"}`, http.StatusOK)

	svcURL, _ := url.Parse(mock.URL)
	service := &Service{Name: "user", URL: svcURL, Prefix: "/user"}
	services := map[string]*Service{"/user": service}

	gw := setupGateway(t, services)
	req := httptest.NewRequest(http.MethodGet, "/user/profile", nil)
	w := httptest.NewRecorder()

	gw.ServeHTTP(w, req)
	resp := w.Result()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d, body: %s", resp.StatusCode, string(body))
	}
	if !bytes.Contains(body, []byte("user service ok")) {
		t.Fatalf("unexpected body: %s", string(body))
	}
}

func TestGateway_MultipleServiceRouting(t *testing.T) {
	userSrv := mockService(t, "user ok", http.StatusOK)
	orderSrv := mockService(t, "order ok", http.StatusOK)

	userURL, _ := url.Parse(userSrv.URL)
	orderURL, _ := url.Parse(orderSrv.URL)

	services := map[string]*Service{
		"/user":  {Name: "user", URL: userURL, Prefix: "/user"},
		"/order": {Name: "order", URL: orderURL, Prefix: "/order"},
	}

	gw := setupGateway(t, services)

	tests := []struct {
		path     string
		expected string
	}{
		{"/user/profile", "user ok"},
		{"/order/checkout", "order ok"},
	}

	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		w := httptest.NewRecorder()

		gw.ServeHTTP(w, req)
		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		if !bytes.Contains(body, []byte(tc.expected)) {
			t.Errorf("expected %q got %q for path %s", tc.expected, string(body), tc.path)
		}
	}
}

func TestGateway_SupportsHTTPMethods(t *testing.T) {
	mock := mockService(t, "method ok", http.StatusOK)
	svcURL, _ := url.Parse(mock.URL)

	service := &Service{Name: "api", URL: svcURL, Prefix: "/api"}
	gw := setupGateway(t, map[string]*Service{"/api": service})

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete}

	for _, m := range methods {
		reqBody := bytes.NewBufferString(`{"data":"test"}`)
		req := httptest.NewRequest(m, "/api/test", reqBody)
		w := httptest.NewRecorder()

		gw.ServeHTTP(w, req)
		resp := w.Result()
		body, _ := io.ReadAll(resp.Body)

		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", m, resp.StatusCode)
		}
		if !bytes.Contains(body, []byte("method ok")) {
			t.Errorf("%s: expected body to contain 'method ok', got %q", m, string(body))
		}
	}
}

func TestGateway_UnknownRoute(t *testing.T) {
	gw := setupGateway(t, map[string]*Service{})
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	w := httptest.NewRecorder()

	gw.ServeHTTP(w, req)
	resp := w.Result()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
