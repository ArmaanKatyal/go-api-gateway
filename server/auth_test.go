package main

import (
	"bytes"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Mock for a failing reader (returns an error on read)
type failingReader struct{}

func (failingReader) Read(p []byte) (n int, err error) {
	return 0, errors.New("read error")
}

func TestAuthNewJwtAuth(t *testing.T) {
	t.Run("valid file input", func(t *testing.T) {
		input := "test_secret_data"
		reader := bytes.NewReader([]byte(input))
		jwtAuth := NewJwtAuth(true, false, []string{"route1", "route2"}, reader)
		assert.True(t, jwtAuth.Enabled)
		assert.False(t, jwtAuth.Anonymous)
		assert.Len(t, jwtAuth.Routes, 2)
		assert.Equal(t, []byte(input), jwtAuth.getSecret())
	})
	t.Run("invalid file input", func(t *testing.T) {
		reader := failingReader{}
		jwtAuth := NewJwtAuth(true, false, []string{"route1", "route2"}, reader)
		assert.True(t, jwtAuth.Enabled)
		assert.False(t, jwtAuth.Anonymous)
		assert.Len(t, jwtAuth.Routes, 2)
		assert.Equal(t, []byte(DEFAULT_SECRET), jwtAuth.getSecret())
	})
}

func TestAuthPathInRoutes(t *testing.T) {
	t.Run("path in routes", func(t *testing.T) {
		input := "test_secret_data"
		reader := bytes.NewReader([]byte(input))
		jwtAuth := NewJwtAuth(true, false, []string{"route1", "route2"}, reader)
		assert.True(t, jwtAuth.pathInRoutes("route1"))
	})
	t.Run("path not in routes", func(t *testing.T) {
		input := "test_secret_data"
		reader := bytes.NewReader([]byte(input))
		jwtAuth := NewJwtAuth(true, false, []string{"route1", "route2"}, reader)
		assert.False(t, jwtAuth.pathInRoutes("route3"))
	})
}

func TestAuthIsEnabled(t *testing.T) {
	input := "test_secret_data"
	reader := bytes.NewReader([]byte(input))
	jwtAuth := NewJwtAuth(true, false, []string{"route1", "route2"}, reader)
	assert.True(t, jwtAuth.IsEnabled())
}

func TestAuthGetSecret(t *testing.T) {
	input := "test_secret_data"
	reader := bytes.NewReader([]byte(input))
	jwtAuth := NewJwtAuth(true, false, []string{"route1", "route2"}, reader)
	assert.Equal(t, []byte(input), jwtAuth.getSecret())
}

func generateRequest(token string, path string) *http.Request {
	req := &http.Request{
		Header: http.Header{
			"Authorization": []string{token},
		},
		URL: &url.URL{
			Path: path,
		},
	}
	return req
}

func TestAuthAuthenticate(t *testing.T) {
	t.Run("path not in routes", func(t *testing.T) {
		j := NewJwtAuth(true, false, []string{"/route1"}, bytes.NewReader([]byte("test")))
		err := j.Authenticate("test", generateRequest("test", "/test/route2"))
		assert.Nil(t, err)
	})
	t.Run("auth disabled", func(t *testing.T) {
		j := NewJwtAuth(false, false, []string{"/route1"}, bytes.NewReader([]byte("test")))
		err := j.Authenticate("test", generateRequest("test", "/test/route1"))
		assert.Nil(t, err)
	})
	t.Run("token missing", func(t *testing.T) {
		j := NewJwtAuth(true, false, []string{"/route1"}, bytes.NewReader([]byte("test")))
		err := j.Authenticate("test", generateRequest("", "/test/route1"))
		assert.Equal(t, ErrTokenMissing, err)
	})
}
