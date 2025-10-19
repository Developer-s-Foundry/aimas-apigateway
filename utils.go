package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

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
	var gatewaySecret = os.Getenv("GATEWAY_SECRET_KEY")
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	encryptKey := fmt.Sprintf("%s:%s", config.Name, timestamp)
	h := hmac.New(sha256.New, []byte(gatewaySecret))
	h.Write([]byte(encryptKey))
	signature := hex.EncodeToString(h.Sum(nil))
	req.Header.Set("X-Gateway-Timestamp", timestamp)
	req.Header.Set("X-Gateway-Signature", signature)
}

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func ValidateJWT(tokenStr string) (*Claims, error) {
	var jwtSecret = os.Getenv("JWT_SECRET")
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}
