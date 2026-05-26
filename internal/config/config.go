package config

import (
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	// Application
	AppPort int    `env:"APP_PORT" envDefault:"8080"`
	AppEnv  string `env:"APP_ENV"  envDefault:"development"`

	// Feature Flags
	CacheEnabled          bool    `env:"CACHE_ENABLED"           envDefault:"false"`
	QueueEnabled          bool    `env:"QUEUE_ENABLED"           envDefault:"false"`
	RateLimitEnabled      bool    `env:"RATE_LIMIT_ENABLED"      envDefault:"false"`
	RateLimitRPS          float64 `env:"RATE_LIMIT_RPS"          envDefault:"100"`
	RateLimitBurst        int     `env:"RATE_LIMIT_BURST"        envDefault:"200"` // TODO: not used yet
	CircuitBreakerEnabled bool    `env:"CIRCUIT_BREAKER_ENABLED" envDefault:"false"`
	CBMaxFailures         int     `env:"CB_MAX_FAILURES"         envDefault:"5"`
	CBTimeoutSeconds      int     `env:"CB_TIMEOUT_SECONDS"      envDefault:"10"`
	DBReadReplicaEnabled  bool    `env:"DB_READ_REPLICA_ENABLED" envDefault:"false"`

	// Database
	DBPrimaryDSN     string `env:"DB_PRIMARY_DSN"`
	PgBouncerDSN     string `env:"PGBOUNCER_DSN"`
	PgBouncerReadDSN string `env:"PGBOUNCER_READ_DSN"`

	// Redis
	RedisAddr        string        `env:"REDIS_ADDR"          envDefault:"redis:6379"`
	CacheBalanceTTL  time.Duration `env:"CACHE_BALANCE_TTL"   envDefault:"10s"`
	CacheTxStatusTTL time.Duration `env:"CACHE_TX_STATUS_TTL" envDefault:"30s"`

	// Queue
	QueueURL     string `env:"QUEUE_URL"`
	QueueWorkers int    `env:"QUEUE_WORKERS" envDefault:"10"`
}

func Load() (Config, error) {
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}
