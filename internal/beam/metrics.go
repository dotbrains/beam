package beam

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

type serverMetrics struct {
	requests         atomic.Int64
	latencyNanos     atomic.Int64
	deliveries       atomic.Int64
	callbackAttempts atomic.Int64
	rateLimited      atomic.Int64
	providerFailures atomic.Int64
}

func (m *serverMetrics) recordDelivery(count int) {
	if count > 0 {
		m.deliveries.Add(int64(count))
	}
}

func (m *serverMetrics) recordCallbackAttempts(count int) {
	if count > 0 {
		m.callbackAttempts.Add(int64(count))
	}
}

type metricsResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *metricsResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func instrumentMetrics(next http.Handler, metrics *serverMetrics) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &metricsResponseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(recorder, r)
		metrics.requests.Add(1)
		metrics.latencyNanos.Add(time.Since(start).Nanoseconds())
		if recorder.status == http.StatusTooManyRequests {
			metrics.rateLimited.Add(1)
		}
		if recorder.status == http.StatusBadGateway {
			metrics.providerFailures.Add(1)
		}
	})
}

func writeMetrics(w http.ResponseWriter, metrics *serverMetrics) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	latency := float64(metrics.latencyNanos.Load()) / float64(time.Second)
	_, _ = fmt.Fprintf(w, "beam_http_requests_total %d\n", metrics.requests.Load())
	_, _ = fmt.Fprintf(w, "beam_http_request_latency_seconds_total %.9f\n", latency)
	_, _ = fmt.Fprintf(w, "beam_deliveries_total %d\n", metrics.deliveries.Load())
	_, _ = fmt.Fprintf(w, "beam_callback_attempts_scheduled_total %d\n", metrics.callbackAttempts.Load())
	_, _ = fmt.Fprintf(w, "beam_http_rate_limited_responses_total %d\n", metrics.rateLimited.Load())
	_, _ = fmt.Fprintf(w, "beam_provider_failures_total %d\n", metrics.providerFailures.Load())
}
