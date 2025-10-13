package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var gatewaySecret = os.Getenv("GATEWAY_SECRET_KEY")

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
	Status     string      `json:"status"`
	Message    string      `json:"message"`
	Data       interface{} `json:"data,omitempty"`
	StatusCode int         `json:"status_code,omitempty"`
	Error      interface{} `json:"error,omitempty"`
}

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

func JSONBadResponse(w http.ResponseWriter, message string, statusCode int, error interface{}) {
	resp := JSONResponse{
		Status:     http.StatusText(statusCode),
		Message:    message,
		StatusCode: statusCode,
	}
	writeJSON(w, statusCode, resp)
}

func signRequest(req *http.Request, config Service) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	encryptKey := fmt.Sprintf("%s:%s", config.Name, timestamp)
	h := hmac.New(sha256.New, []byte(gatewaySecret))
	h.Write([]byte(encryptKey))
	signature := hex.EncodeToString(h.Sum(nil))
	req.Header.Set("X-Gateway-Timestamp", timestamp)
	req.Header.Set("X-Gateway-Signature", signature)
	req.Header.Set("X-Gateway-Service", "gateway-main")
}
