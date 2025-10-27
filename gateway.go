package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

type Gateway struct {
	atomicRoutes atomic.Value

	rateLimiter *RateLimiter
	proxyCache  sync.Map
	mu          sync.Mutex
	logger      *Log
}

func main() {
	godotenv.Load()
	logger := NewLogger()

	configFile := flag.String("config", "aimas.yml", "configuration file path")
	flag.Parse()

	gw := NewGateway(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := gw.reloadFromPath(*configFile); err != nil {
		logger.Fatal("config", fmt.Sprintf("failed to load config: %v", err), err)
	}

	if err := gw.WatchConfig(*configFile, ctx); err != nil {
		logger.Fatal("config", fmt.Sprintf("failed to watch config: %v", err), err)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: gw,
	}

	go func() {
		logger.Info("gateway", fmt.Sprintf("gateway starting on %s", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server", fmt.Sprintf("listen error: %v", err), err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	logger.Info("server", "shutting down...")
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		logger.Fatal("server", fmt.Sprintf("shutdown error: %v", err), err)
	}
	logger.Info("server", "server stopped")
}

func NewGateway(logger *Log) *Gateway {
	g := &Gateway{logger: logger, rateLimiter: NewRateLimiter()}
	g.atomicRoutes.Store(map[string]*Service{})
	return g
}

func (g *Gateway) getReverseProxy(svc *Service) *httputil.ReverseProxy {
	if v, ok := g.proxyCache.Load(svc.Name); ok {
		return v.(*httputil.ReverseProxy)
	}

	g.mu.Lock()
	defer g.mu.Unlock()
	if v, ok := g.proxyCache.Load(svc.Name); ok {
		return v.(*httputil.ReverseProxy)
	}

	target := svc.URL

	director := func(req *http.Request) {
		origPath := req.URL.Path
		prefix := svc.Prefix

		var trimmed string

		if svc.StripPefix {
			trimmed = strings.TrimPrefix(origPath, prefix)
			if trimmed == "" {
				trimmed = "/"
			}
		} else {
			trimmed = origPath
		}

		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.URL.Path = trimmed

		// origHeaders := req.Header.Clone()

		// req.Header = make(http.Header)
		// for k, v := range origHeaders {
		// 	for _, hv := range v {
		// 		req.Header.Add(k, hv)
		// 	}
		// }

		if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
			if prior := req.Header.Get("X-Forwarded-For"); prior != "" {
				req.Header.Set("X-Forwarded-For", prior+", "+clientIP)
			} else {
				req.Header.Set("X-Forwarded-For", clientIP)
			}
		}

		signRequest(req, *svc)

		req.Host = target.Host

	}

	proxy := &httputil.ReverseProxy{
		Director: director,
		ErrorHandler: func(w http.ResponseWriter, req *http.Request, err error) {
			g.logger.Error("proxy-error",
				fmt.Sprintf("proxy error for service %s: %v", svc.Name, err),
				err,
			)
			message := map[string]interface{}{
				"message":     fmt.Sprintf("failed to reach service %s", svc.Name),
				"error":       err.Error(),
				"status_code": http.StatusBadGateway,
			}
			JSONBadResponse(w, "bad gateway", http.StatusBadGateway, message)
		},
	}

	g.proxyCache.Store(svc.Name, proxy)
	return proxy
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/health" || r.URL.Path == "/healthz" {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
		return
	}

	r.Header.Set("X-Request-ID", uuid.NewString())
	routes := g.atomicRoutes.Load().(map[string]*Service)
	prefix := extractPrefix(r.URL.Path)
	svc, ok := routes[prefix]
	if !ok {
		JSONBadResponse(w, "service not found", http.StatusNotFound, nil)
		return
	}

	proxy := g.getReverseProxy(svc)

	h := applyMiddleWare(
		http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			proxy.ServeHTTP(w, req)
		}),
		LoggingMiddleware(*svc, g.logger),
		g.rateLimiter.Middleware(svc.Name, svc.RateLimit.RequestsPerMinute),
		g.AuthMiddleware,
		RecoverMiddleware,
		SecurityHeadersMiddleware,
	)
	h.ServeHTTP(w, r)
}
