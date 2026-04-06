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
	depCacheLookups = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Help: "Total number of dependency cache lookups by result",
			Name: "dep_cache_lookups_total",
		},
		[]string{"result"},
	)

	depInstallDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Buckets: []float64{0.5, 1, 2.5, 5, 10, 30, 60},
			Help:    "Duration of bun install in seconds",
			Name:    "dep_install_duration_seconds",
		},
	)

	compileDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			Help:    "Duration of TypeScript compilation in seconds",
			Name:    "compile_duration_seconds",
		},
	)

	compileResults = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Help: "Total number of compile operations by result",
			Name: "compile_results_total",
		},
		[]string{"result"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			Help:    "Duration of HTTP requests in seconds",
			Name:    "http_request_duration_seconds",
		},
		[]string{"endpoint", "method"},
	)

	packageResolutions = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Help: "Total number of package resolutions by package name",
			Name: "package_resolutions_total",
		},
		[]string{"package"},
	)

	requestCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Help: "Total number of HTTP requests",
			Name: "http_requests_total",
		},
		[]string{"endpoint", "method", "status"},
	)

	typecheckDuration = prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
			Help:    "Duration of TypeScript type checking in seconds",
			Name:    "typecheck_duration_seconds",
		},
	)

	typecheckResults = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Help: "Total number of typecheck operations by result",
			Name: "typecheck_results_total",
		},
		[]string{"result"},
	)
)

func init() {
	prometheus.MustRegister(depCacheLookups)
	prometheus.MustRegister(depInstallDuration)
	prometheus.MustRegister(compileDuration)
	prometheus.MustRegister(compileResults)
	prometheus.MustRegister(httpRequestDuration)
	prometheus.MustRegister(packageResolutions)
	prometheus.MustRegister(requestCounter)
	prometheus.MustRegister(typecheckDuration)
	prometheus.MustRegister(typecheckResults)
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
