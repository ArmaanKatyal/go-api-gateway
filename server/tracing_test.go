package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTracingToList(t *testing.T) {
	m := MetricsInput{
		Code:   "testcode",
		Method: "testmethod",
		Route:  "testroute",
	}
	assert.Equal(t, []string{"testcode", "testmethod", "testroute"}, m.ToList())
}

func TestTracingNewPromMetrics(t *testing.T) {
	t.Run("metrics prefix match", func(t *testing.T) {
		AppConfig.Server.Metrics.Prefix = "testing"
		p := NewPromMetrics()
		assert.Equal(t, "testing", p.prefix)
	})
}

func TestTracingGetLabels(t *testing.T) {
	assert.Equal(t, []string{"Code", "Method", "Route"}, getLabels())
}
