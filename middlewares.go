package main

import (
	"net/http"
	"runtime/debug"
	"time"

	"github.com/rs/zerolog/hlog"
)

type MiddleWare func(http.Handler) http.Handler

func applyMiddleWare(handler http.Handler, middlewares ...MiddleWare) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}
	return handler
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
				JSONBadResponse(w, "internal server error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
