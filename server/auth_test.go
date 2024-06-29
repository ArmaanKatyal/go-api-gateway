package main

import (
	"bytes"
	"errors"
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

func TestAuthAuthenticate(t *testing.T) {

}
