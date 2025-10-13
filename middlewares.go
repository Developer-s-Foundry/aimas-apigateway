package main

import (
	"fmt"
	"net/http"
	"runtime/debug"
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
		rpm = 60
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
					retryAfter = 1 * time.Second // fallback durh
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

func LoggingMiddleware(config Service) func(http.Handler) http.Handler {
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
						Str("user_agent", r.UserAgent()).
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
				log.lg.Error().
					Str("path", r.URL.Path).
					Str("method", r.Method).
					Interface("panic", rec).
					Str("stack", string(debug.Stack())).
					Msg("panic recovered from handler")

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				JSONBadResponse(w, "internal server error", http.StatusInternalServerError, nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		w.Header().Set("Referrer-Policy", "no-referrer-when-downgrade")
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}
