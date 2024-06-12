package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	jwt.RegisteredClaims
}

type ContextKey string
type AuthError error

var (
	ErrTokenMissing AuthError = errors.New("missing auth token")
	ErrInvalidToken AuthError = errors.New("invalid auth token")
)

type JwtAuth struct {
	Enabled bool     `json:"enabled"`
	Routes  []string `json:"routes"`
}

// Authenticate checks if the request has a valid JWT token in the header
func (j *JwtAuth) Authenticate(name string, r *http.Request) AuthError {
	token := r.Header.Get("Authorization")
	path := "/" + strings.Split(r.URL.Path, "/")[2]
	slog.Info("Authenticating request", "service", name, "path", path)
	exists := j.pathInRoutes(path)
	if exists {
		if token == "" {
			return ErrTokenMissing
		}
		// parse token
		claims := &Claims{}
		// TODO: fetch/use a custom secret for each service
		parsed, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte("test"), nil
		})
		if err != nil {
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

func NewJwtAuth(enabled bool, routes []string) *JwtAuth {
	return &JwtAuth{
		Enabled: enabled,
		Routes:  routes,
	}
}
