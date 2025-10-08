package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

var log = NewLogger()

func main() {
	godotenv.Load()
	sc, err := NewServiceConfig("aimas.yml", "")
	if err != nil {
		log.Error("service-config", err)
	}

	gateWayServer(sc.Services...)

	port := os.Getenv("PORT")

	log.Info("message", fmt.Sprintf("server running on port: %s", port))
	log.Fatal("server-err", http.ListenAndServe(fmt.Sprintf(":%s", port), nil))
}

func gateWayServer(serverConfig ...Service) {
	for _, config := range serverConfig {
		if err := config.parseURL(); err != nil {
			log.Error("config-error", err)
			continue
		}

		targetServer, err := url.Parse(config.Host)
		if err != nil {
			log.Error("target-server", err)
			continue
		}

		for _, route := range config.Routes {
			var fullPath string
			if config.Prefix != "" {
				fullPath = path.Join(config.Prefix, route.Path)
			} else {
				fullPath = route.Path
			}

			h := applyMiddleWare(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				proxyRouter(w, r, targetServer, config, route)
			}), LoggingMiddleware(config))
			http.Handle(fullPath, h)
		}
	}
}

func proxyRouter(w http.ResponseWriter, r *http.Request, targetServer *url.URL, config Service, route Route) {
	if !contains(route.Methods, r.Method) {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
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
