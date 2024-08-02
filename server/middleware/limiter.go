package middleware

import (
	"github.com/ArmaanKatyal/go_api_gateway/server/feature"
	"log/slog"
	"net/http"
)

func RateLimiterMiddleware(limiter *feature.GlobalRateLimiter) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if limiter.IsEnabled() {
				v := limiter.GetVisitor(r.RemoteAddr)
				if !v.Limiter.Allow() {
					slog.Error("Rate limit exceeded", "path", r.URL.Path, "method", r.Method, "ip", r.RemoteAddr)
					http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
					return
				}
			}
			next(w, r)
		}
	}
}
