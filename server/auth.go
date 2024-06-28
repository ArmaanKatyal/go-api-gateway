package main

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Service string `json:"service"`
	jwt.RegisteredClaims
}

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

		c, err := json.Marshal(claims)
		if err != nil {
			slog.Error("Error marshalling claims", "error", err.Error(), "service", name, "path", path)
			return err
		}

		// Append claims to Header
		r.Header.Add("X-Claims", string(c))
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

func NewJwtAuth(enabled bool, anonymous bool, routes []string, reader io.Reader) *JwtAuth {
	if reader != nil {
		data, err := io.ReadAll(reader)
		if err != nil {
			slog.Debug("Error reading secret file", "error", err.Error())
			data = []byte(DEFAULT_SECRET)
		}
		return &JwtAuth{
			Enabled:   enabled,
			Anonymous: anonymous,
			Routes:    routes,
			secret:    data,
		}
	}
	return &JwtAuth{
		Enabled:   enabled,
		Anonymous: anonymous,
		Routes:    routes,
		secret:    []byte(DEFAULT_SECRET),
	}
}
