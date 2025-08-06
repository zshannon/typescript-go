package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/evanw/esbuild/pkg/api"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Prometheus metrics
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"endpoint", "method"},
	)

	typecheckDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "typecheck_duration_seconds",
			Help:    "Duration of TypeScript type checking in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
	)

	compileDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "compile_duration_seconds",
			Help:    "Duration of TypeScript compilation in seconds",
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
	)

	packageResolutions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "package_resolutions_total",
			Help: "Total number of package resolutions by package name",
		},
		[]string{"package"},
	)

	requestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"endpoint", "method", "status"},
	)

	typecheckResults = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "typecheck_results_total",
			Help: "Total number of typecheck operations by result",
		},
		[]string{"result"},
	)

	compileResults = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "compile_results_total",
			Help: "Total number of compile operations by result",
		},
		[]string{"result"},
	)
)

func init() {
	// Register Prometheus metrics
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(typecheckDuration)
	prometheus.MustRegister(compileDuration)
	prometheus.MustRegister(packageResolutions)
	prometheus.MustRegister(requestCounter)
	prometheus.MustRegister(typecheckResults)
	prometheus.MustRegister(compileResults)
}

// trackPackageResolution extracts and tracks package name from import path
func trackPackageResolution(path string) {
	if !strings.HasPrefix(path, "/") && !strings.HasPrefix(path, ".") {
		// This is a package import
		packageName := path
		if idx := strings.Index(path, "/"); idx > 0 {
			if strings.HasPrefix(path, "@") {
				// Scoped package like @crayonnow/core
				if secondSlash := strings.Index(path[idx+1:], "/"); secondSlash > 0 {
					packageName = path[:idx+1+secondSlash]
				} else {
					packageName = path
				}
			} else {
				// Regular package like react/jsx-runtime
				packageName = path[:idx]
			}
		}
		packageResolutions.WithLabelValues(packageName).Inc()
	}
}

// wrapResolverWithMetrics wraps the resolver function to track package resolutions
func wrapResolverWithMetrics(resolver func(string) (api.OnLoadResult, error)) func(string) (api.OnLoadResult, error) {
	return func(path string) (api.OnLoadResult, error) {
		trackPackageResolution(path)
		return resolver(path)
	}
}

// recordHTTPMetrics records HTTP request metrics
func recordHTTPMetrics(r *http.Request, statusCode int, duration time.Duration) {
	httpRequestDuration.WithLabelValues(r.URL.Path, r.Method).Observe(duration.Seconds())
	requestCounter.WithLabelValues(r.URL.Path, r.Method, fmt.Sprintf("%d", statusCode)).Inc()
}

// startMetricsServer starts the Prometheus metrics server on port 9091
func startMetricsServer() {
	metricsServer := &http.Server{
		Addr:    ":9091",
		Handler: promhttp.Handler(),
	}
	log.Printf("Metrics server listening on :9091/metrics")
	if err := metricsServer.ListenAndServe(); err != nil {
		log.Printf("Metrics server error: %v", err)
	}
}