package main

import (
	"fmt"
	"reflect"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type PromMetrics struct {
	prefix                    string
	httpTransactionTotal      *prometheus.CounterVec
	httpResponseTimeHistogram *prometheus.HistogramVec
	buckets                   []float64
}

type MetricsInput struct {
	Code   string
	Method string
	Route  string
}

func (m *MetricsInput) ToList() []string {
	var values []string
	inputValue := reflect.ValueOf(*m)

	for i := 0; i < inputValue.NumField(); i++ {
		value := inputValue.Field(i)
		values = append(values, fmt.Sprint(value.Interface()))
	}

	return values
}

func getLabels() []string {
	var labels []string
	metricsInputType := reflect.TypeOf(MetricsInput{})
	for i := 0; i < metricsInputType.NumField(); i++ {
		labels = append(labels, metricsInputType.Field(i).Name)
	}
	return labels
}

func NewPromMetrics() *PromMetrics {
	prefix := AppConfig.Server.Metrics.Prefix
	return &PromMetrics{
		prefix: prefix,
		httpTransactionTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: prefix + "_requests_total",
			Help: "Total HTTP requests processed",
		}, getLabels()),
		httpResponseTimeHistogram: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name: prefix + "_response_time_seconds",
			Help: "Histogram of response time for handler",
		}, getLabels()),
		buckets: AppConfig.Server.Metrics.Buckets,
	}
}

func (pm *PromMetrics) ObserveResponseTime(input MetricsInput, time float64) {
	pm.httpResponseTimeHistogram.WithLabelValues(input.ToList()...).Observe(time)
}

func (pm *PromMetrics) IncHttpTransaction(input MetricsInput, time float64) {
	pm.httpTransactionTotal.WithLabelValues(input.ToList()...).Inc()
}
