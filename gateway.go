package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

var log = NewLogger()

func main() {
	godotenv.Load()

	configFile := flag.String("config", "aimas.yml", "user configuration file")
	path := flag.String("path", "", "config file path")
	flag.Parse()

	sc, err := NewServiceConfig(*configFile, *path)
	if err != nil {
		log.Error("service-config", err)
	}

	router := gateWayServer(sc.Services...)

	port := os.Getenv("PORT")

	log.Info("message", fmt.Sprintf("server running on port: %s", port))
	log.Fatal("server-err", http.ListenAndServe(fmt.Sprintf(":%s", port), router))
}

func gateWayServer(serverConfig ...Service) http.Handler {
	router := mux.NewRouter()
	for _, config := range serverConfig {
		if err := config.parseURL(); err != nil {
			log.lg.Error().
				Err(err).
				Str("service", config.Name).
				Msg("failed to parse service URL")
			continue
		}

		targetServer, err := url.Parse(config.Host)
		if err != nil || targetServer.Scheme == "" || targetServer.Host == "" {
			log.lg.Error().
				Err(err).
				Str("service", config.Name).
				Str("host", config.Host).
				Msg("invalid target server URL")
			continue
		}

		for _, route := range config.Routes {
			var fullPath string
			if config.Prefix != "" {
				fullPath = path.Join(config.Prefix, route.Path)
			} else {
				fullPath = route.Path
			}

			r := NewRateLimiter()
			h := applyMiddleWare(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				proxyRouter(w, r, targetServer, config)
			}),
				r.Middleware(config.Name, config.RateLimit.RequestsPerMinute),
				LoggingMiddleware(config),
				RecoverMiddleware,
			)

			router.Handle(fullPath, h).Methods(route.Methods...)
		}
	}

	router.NotFoundHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		JSONBadResponse(w, "404 not found", http.StatusNotFound, nil)
	})

	router.MethodNotAllowedHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		JSONBadResponse(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed, nil)
	})

	return router
}

func proxyRouter(w http.ResponseWriter, r *http.Request, targetServer *url.URL, config Service) {
	var body io.Reader
	if r.Body != nil {
		data, _ := io.ReadAll(r.Body)
		r.Body.Close()
		body = bytes.NewReader(data)
	}
	proxyURL := targetServer.String() + r.URL.Path
	setReqHeaders(r, targetServer, config)
	req, err := http.NewRequest(r.Method, proxyURL, body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.Header = r.Header.Clone()

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if isStreamingResponse(resp) {
		streamResponse(w, resp)
	} else {
		forwardResponse(w, resp)
	}
}

func setReqHeaders(r *http.Request, targetServer *url.URL, config Service) {
	r.Host = targetServer.Host
	r.URL.Host = targetServer.Host
	r.URL.Scheme = targetServer.Scheme
	r.RequestURI = ""
	ip, err := config.clientIp(r.RemoteAddr)
	if err != nil {
		log.lg.Err(err).AnErr("client-ip", err)
	}
	r.Header.Set("X-Forwarded-Proto", config.Protocol)
	r.Header.Set("User-Agent", "aimas-gateway/1.0")
	r.Header.Set("X-Request-ID", uuid.NewString())
	r.Header.Set("Authorization", "Bearer internal_service_token_123") //TODO: will implement this later on
	r.Header.Set("X-Forwarded-For", ip)
}

func streamResponse(w http.ResponseWriter, resp *http.Response) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}
	buf := make([]byte, 1024)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			flusher.Flush()
		}
		if err != nil {
			if err != io.EOF {
				log.Error("eof", err)
			}
			break
		}
	}
}
