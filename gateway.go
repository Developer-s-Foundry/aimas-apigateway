package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

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
			panic(err)
		}
		targetServer, err := url.Parse(config.Host)
		if err != nil {
			log.Println(err)
			return
		}
		proxy := httputil.NewSingleHostReverseProxy(targetServer)
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			setReqHeaders(r, targetServer, config)

			proxy.ServeHTTP(w, r)
		})
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
	r.Header.Set("Authorization", "Bearer internal_service_token_123") // will implement this later on
	r.Header.Set("X-Forwarded-For", ip)
}
