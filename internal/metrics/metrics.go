package metrics

import "github.com/prometheus/client_golang/prometheus"

var (
	CacheHits = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_hits_total",
		Help: "Total number of Redis cache hits.",
	}, []string{"key_type"})

	CacheMisses = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cache_misses_total",
		Help: "Total number of Redis cache misses.",
	}, []string{"key_type"})

	RateLimiterDrops = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "rate_limiter_drops_total",
		Help: "Total number of requests rejected by the rate limiter.",
	}, []string{"scope"})

	QueueDepth = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "queue_depth",
		Help: "Number of messages waiting in the queue.",
	}, []string{"queue"})

	CircuitBreakerOpen = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "circuit_breaker_open",
		Help: "1 when the circuit breaker is open, 0 when closed.",
	})

	CircuitBreakerState = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "circuit_breaker_state",
		Help: "Circuit breaker state by dependency: 0=closed, 1=open, 2=half_open.",
	}, []string{"dependency"})

	DependencyUp = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "dependency_up",
		Help: "Health status for dependencies: 1=up, 0=down.",
	}, []string{"dependency"})

	DBConnectionsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_connections_active",
		Help: "Number of active (in-use) database connections.",
	})

	DBConnectionsIdle = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_connections_idle",
		Help: "Number of idle database connections in the pool.",
	})

	AppCPUUtilizationRatio = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "app_cpu_utilization_ratio",
		Help: "Approximate CPU utilization ratio for the API process, normalized by GOMAXPROCS.",
	})

	AppMemoryUsageBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "app_memory_usage_bytes",
		Help: "Approximate memory reserved by the API process in bytes.",
	})

	AppMemoryLimitBytes = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "app_memory_limit_bytes",
		Help: "Memory limit used to compute app memory utilization in bytes.",
	})

	AppMemoryUtilizationRatio = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "app_memory_utilization_ratio",
		Help: "Approximate memory utilization ratio for the API process.",
	})
)

func init() {
	prometheus.MustRegister(
		CacheHits,
		CacheMisses,
		RateLimiterDrops,
		QueueDepth,
		CircuitBreakerOpen,
		CircuitBreakerState,
		DependencyUp,
		DBConnectionsActive,
		DBConnectionsIdle,
		AppCPUUtilizationRatio,
		AppMemoryUsageBytes,
		AppMemoryLimitBytes,
		AppMemoryUtilizationRatio,
	)

	QueueDepth.WithLabelValues("transactions").Set(0)
	CircuitBreakerState.WithLabelValues("api").Set(0)
	DependencyUp.WithLabelValues("api").Set(1)
	DependencyUp.WithLabelValues("database").Set(0)
	DependencyUp.WithLabelValues("redis").Set(0)
	DependencyUp.WithLabelValues("rabbitmq").Set(0)
}
