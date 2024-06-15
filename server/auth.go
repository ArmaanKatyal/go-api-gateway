package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	jwt.RegisteredClaims
}

type ContextKey string
type AuthError error

const (
	DEFAULT_SECRET = "test"
)

var (
	ErrTokenMissing AuthError = errors.New("missing auth token")
	ErrInvalidToken AuthError = errors.New("invalid auth token")
)

type JwtAuth struct {
	Enabled   bool     `json:"enabled"`
	Anonymous bool     `json:"anonymous"`
	Routes    []string `json:"routes"`
	secret    []byte
}

func (j *JwtAuth) getSecret() []byte {
	return j.secret
}

// Authenticate checks if the request has a valid JWT token in the header
func (j *JwtAuth) Authenticate(name string, r *http.Request) AuthError {
	token := r.Header.Get("Authorization")
	path := "/" + strings.Split(r.URL.Path, "/")[2]
	slog.Info("Authenticating request", "service", name, "path", path)
	exists := j.pathInRoutes(path)
	if exists && j.IsEnabled() {
		if token == "" {
			if j.Anonymous {
				slog.Warn("Anonymous request", "service", name, "path", path)
				return nil
			}
			return ErrTokenMissing
		}
		// parse token
		claims := &Claims{}
		parsed, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
			return j.getSecret(), nil
		})
		if err != nil {
			if j.Anonymous {
				slog.Warn("Anonymous request", "service", name, "path", path)
				return nil
			}
			slog.Error("Error parsing token", "error", err.Error(), "service", name, "path", path)
			return err
		}
		if !parsed.Valid {
			slog.Error("Invalid token", "service", name, "path", path)
			return ErrInvalidToken
		}

		// Check expiration
		if claims.ExpiresAt.Unix() < time.Now().Unix() {
			slog.Error("Token expired", "service", name, "path", path)
			if j.Anonymous {
				slog.Warn("Anonymous request", "service", name, "path", path)
				return nil
			}
			return ErrInvalidToken
		}

		// Append claims to context
		ctx := context.WithValue(r.Context(), ContextKey("claims"), claims)
		*r = *r.WithContext(ctx)
	}
	return nil
}

func (j *JwtAuth) pathInRoutes(path string) bool {
	for _, route := range j.Routes {
		if route == path {
			return true
		}
	}
	return false
}

func (j *JwtAuth) IsEnabled() bool {
	return j.Enabled
}

func NewJwtAuth(enabled bool, anonymous bool, routes []string, path string) *JwtAuth {
	data, err := os.ReadFile(path)
	if err != nil {
		slog.Error("Error reading secret file", "error", err.Error(), "path", path)
		data = []byte(DEFAULT_SECRET)
	}
	return &JwtAuth{
		Enabled:   enabled,
		Anonymous: anonymous,
		Routes:    routes,
		secret:    data,
	}
}
