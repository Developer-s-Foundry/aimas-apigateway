package main

import (
	"net/http"
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
				case status < 400:
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
