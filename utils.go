package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func contains(methods []string, method string) bool {
	for _, m := range methods {
		fmt.Println(m, method)
		if m == method {
			return true
		}
	}
	return false
}

func forwardResponse(w http.ResponseWriter, resp *http.Response) {
	copyHeaders(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func isStreamingResponse(resp *http.Response) bool {
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	te := strings.ToLower(resp.Header.Get("Transfer-Encoding"))

	return strings.Contains(te, "chunked") ||
		strings.Contains(ct, "text/event-stream") ||
		(resp.ContentLength == -1 && resp.Header.Get("Content-Length") == "")
}

func copyHeaders(dst, src http.Header) {
	for k, v := range src {
		for _, vv := range v {
			dst.Add(k, vv)
		}
	}
}
