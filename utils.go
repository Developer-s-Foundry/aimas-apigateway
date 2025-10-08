package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func contains(methods []string, method string) bool {
	for _, m := range methods {
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

type JSONResponse struct {
	Status     string      `json:"status"`                // "success" or "bad"
	Message    string      `json:"message"`               // description of the response
	Data       interface{} `json:"data,omitempty"`        // present for success only
	StatusCode int         `json:"status_code,omitempty"` // useful for bad responses
}

// writeJSON handles encoding and writing headers
func writeJSON(w http.ResponseWriter, statusCode int, resp JSONResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}

func JSONSuccess(w http.ResponseWriter, message string, data interface{}, statusCode int) {
	resp := JSONResponse{
		Status:  "success",
		Message: message,
		Data:    data,
	}
	writeJSON(w, statusCode, resp)
}

func JSONBadResponse(w http.ResponseWriter, message string, statusCode int) {
	resp := JSONResponse{
		Status:     http.StatusText(statusCode),
		Message:    message,
		StatusCode: statusCode,
	}
	writeJSON(w, statusCode, resp)
}
