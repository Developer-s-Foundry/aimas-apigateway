package main

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"path"

	"github.com/google/uuid"
)

func main() {
	sc, err := NewServiceConfig("aimas.yml", "")
	if err != nil {
		log.Println(err)
	}
	gateWayServer(sc.Services...)
	http.ListenAndServe(":8080", nil)
}

func gateWayServer(serverConfig ...Service) {
	for _, config := range serverConfig {
		if err := config.parseURL(); err != nil {
			log.Println("invalid config:", err)
			continue
		}

		targetServer, err := url.Parse(config.Host)
		if err != nil {
			log.Println(err)
			continue
		}

		for _, route := range config.Routes {
			var fullPath string
			if config.Prefix != "" {
				fullPath = path.Join(config.Prefix, route.Path)
			} else {
				fullPath = route.Path
			}

			http.HandleFunc(fullPath, func(w http.ResponseWriter, r *http.Request) {
				if !contains(route.Methods, r.Method) {
					http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
					return
				}

				proxyURL := targetServer.String() + r.URL.Path
				req, err := http.NewRequest(r.Method, proxyURL, r.Body)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				setReqHeaders(r, targetServer, config)
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
			})
		}
	}
}

func setReqHeaders(r *http.Request, targetServer *url.URL, config Service) {
	r.Host = targetServer.Host
	r.URL.Host = targetServer.Host
	r.URL.Scheme = targetServer.Scheme
	r.RequestURI = ""
	ip, err := config.clientIp(r.RemoteAddr)
	if err != nil {
		log.Println(err)
	}
	r.Header.Set("X-Forwarded-Proto", config.Protocol)
	r.Header.Set("User-Agent", "aimas-gateway/1.0")
	r.Header.Set("X-Correlation-ID", uuid.New().String())
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
				log.Println("Stream error:", err)
			}
			break
		}
	}
}
