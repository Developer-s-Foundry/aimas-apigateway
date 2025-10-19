package main

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/rs/zerolog/hlog"
	"golang.org/x/time/rate"
)

type MiddleWare func(http.Handler) http.Handler

func applyMiddleWare(handler http.Handler, middlewares ...MiddleWare) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler

}

func (r *RateLimiter) Middleware(serviceName string, rpm int) func(http.Handler) http.Handler {
	if rpm <= 0 {
		rpm = 120
	}
	limit := rate.Every(time.Minute / time.Duration(rpm))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {

			rate_ley := extractClientID(req)
			limiter := r.getRateLimiter(rate_ley, limit, rpm)
			if !limiter.Allow() {
				reserve := limiter.Reserve()
				retryAfter := reserve.Delay()
				if retryAfter < 0 {
					retryAfter = 1 * time.Second
				}
				w.Header().Set("Retry-After", fmt.Sprintf("%.0f", retryAfter.Seconds()))
				errMsg := fmt.Sprintf("retry after second %.0f", retryAfter.Seconds())
				JSONBadResponse(w, "rate limit exceeded", http.StatusTooManyRequests, errMsg)
				return
			}
			next.ServeHTTP(w, req)
		})
	}
}

func LoggingMiddleware(config Service, log *Log) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return hlog.NewHandler(log.lg)(
			hlog.AccessHandler(func(r *http.Request, status, size int, duration time.Duration) {
				logger := hlog.FromRequest(r)
				switch {
				case status >= 100 && status < 400:
					logger.Info().
						Str("method", r.Method).
						Str("path", r.URL.Path).
						Int("status_code", status).
						Str("latency", duration.String()).
						Str("service_target", config.Name).
						Str("request_id", r.Header.Get("X-Request-ID")).
						Msg("request forwarded successfully")

				case status < 500:
					logger.Warn().
						Str("method", r.Method).
						Str("path", r.URL.Path).
						Int("status_code", status).
						Str("latency", duration.String()).
						Str("service_target", config.Name).
						Str("user_agent", r.UserAgent()).
						Str("request_id", r.Header.Get("X-Request-ID")).
						Msg("client error occurred")
				default:
					logger.Error().
						Str("method", r.Method).
						Str("path", r.URL.Path).
						Int("status_code", status).
						Str("latency", duration.String()).
						Str("service_target", config.Name).
						Str("user_agent", r.UserAgent()).
						Str("request_id", r.Header.Get("X-Request-ID")).
						Msg("unexpected server error")
				}
			})(next),
		)
	}
}

func RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic recovered: %v\n%s", rec, string(debug.Stack()))
				JSONBadResponse(w, "internal server error", http.StatusInternalServerError, nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer-when-downgrade")
		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/auth") {
			next.ServeHTTP(w, r)
			return
		}

		tokenStr := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if tokenStr == "" {
			JSONBadResponse(w, "missing token", http.StatusUnauthorized, nil)
			return
		}

		claims, err := ValidateJWT(tokenStr)
		if err != nil {
			fmt.Println(err.Error())
			JSONBadResponse(w, "invalid or expired token", http.StatusUnauthorized, nil)
			return
		}

		r.Header.Set("X-User-ID", claims.UserID)

		next.ServeHTTP(w, r)
	})
}
