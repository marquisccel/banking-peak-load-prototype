package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ahargunyllib/banking-peak-load-prototype/internal/config"
	"github.com/ahargunyllib/banking-peak-load-prototype/internal/domain/account"
	"github.com/ahargunyllib/banking-peak-load-prototype/internal/domain/transaction"
	"github.com/ahargunyllib/banking-peak-load-prototype/internal/handler"
	infrapostgres "github.com/ahargunyllib/banking-peak-load-prototype/internal/infrastructure/postgres"
	infraqueue "github.com/ahargunyllib/banking-peak-load-prototype/internal/infrastructure/queue"
	infraredis "github.com/ahargunyllib/banking-peak-load-prototype/internal/infrastructure/redis"
	"github.com/ahargunyllib/banking-peak-load-prototype/internal/logger"
	"github.com/ahargunyllib/banking-peak-load-prototype/internal/metrics"
	appmw "github.com/ahargunyllib/banking-peak-load-prototype/internal/middleware"
	"github.com/ahargunyllib/banking-peak-load-prototype/internal/repository/memory"
	pgrepo "github.com/ahargunyllib/banking-peak-load-prototype/internal/repository/postgres"
	"github.com/ahargunyllib/banking-peak-load-prototype/internal/service"
	"github.com/ahargunyllib/banking-peak-load-prototype/internal/worker"
	"github.com/jmoiron/sqlx"
	echoprometheus "github.com/labstack/echo-prometheus"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/redis/go-redis/v9"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
		os.Exit(1)
	}

	logger.Init(cfg.AppEnv)

	if cfg.DBPrimaryDSN != "" {
		if err := infrapostgres.RunMigrations(cfg.DBPrimaryDSN); err != nil {
			fmt.Fprintf(os.Stderr, "migrations failed: %v\n", err)
			os.Exit(1)
		}
	}

	var accountRepo account.Repository = memory.NewAccountRepository()
	var txRepo transaction.Repository = memory.NewTransactionRepository()

	var db *sqlx.DB
	if cfg.DBPrimaryDSN != "" {
		appDSN := cfg.DBPrimaryDSN
		if cfg.PgBouncerDSN != "" {
			appDSN = cfg.PgBouncerDSN
		}
		db, err = infrapostgres.New(appDSN)
		if err != nil {
			fmt.Fprintf(os.Stderr, "failed to connect to postgres: %v\n", err)
			os.Exit(1)
		}
		defer func() { _ = db.Close() }()

		var replicaDB *sqlx.DB
		if cfg.DBReadReplicaEnabled && cfg.PgBouncerReadDSN != "" {
			replicaDB, err = infrapostgres.New(cfg.PgBouncerReadDSN)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: read replica unavailable, falling back to primary: %v\n", err)
				replicaDB = nil
			} else {
				defer func() { _ = replicaDB.Close() }()
			}
		}

		accountRepo = pgrepo.NewAccountRepository(db, replicaDB)
		txRepo = pgrepo.NewTransactionRepository(db, replicaDB)
	}

	var redisClient *redis.Client
	if cfg.CacheEnabled && cfg.RedisAddr != "" {
		redisClient, err = infraredis.New(cfg.RedisAddr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: redis unavailable: %v\n", err)
		}
	}

	var queueClient *infraqueue.Client
	if cfg.QueueEnabled && cfg.QueueURL != "" {
		queueClient, err = infraqueue.New(cfg.QueueURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: queue unavailable: %v\n", err)
		} else {
			defer queueClient.Close()
		}
	}

	accountSvc := service.NewAccountService(accountRepo, redisClient, cfg.CacheBalanceTTL)
	txSvc := service.NewTransactionService(txRepo, db, queueClient, redisClient, cfg.CacheTxStatusTTL)

	accountHandler := handler.NewAccountHandler(accountSvc)
	txHandler := handler.NewTransactionHandler(txSvc)

	e := echo.New()
	e.Logger = logger.L                          // route Echo's internal logs through our slog JSON logger
	e.Use(echoprometheus.NewMiddleware("banking_api")) // gather HTTP metrics for all later middleware, including 429s
	e.Use(middleware.BodyLimit(2_097_152))       // 2MB
	e.Use(middleware.ContextTimeout(60 * time.Second))
	// e.Use(middleware.CORS("https://example.com")) // Allow CORS from frontend domain in real deployment
	// e.Use(middleware.CSRF()) // Enable in real deployment with proper config (cookie name, same-site, etc.)
	e.Use(middleware.Decompress())
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Level: 5,
	}))
	if cfg.RateLimitEnabled {
		e.Use(middleware.RateLimiterWithConfig(middleware.RateLimiterConfig{
			Store: middleware.NewRateLimiterMemoryStoreWithConfig(middleware.RateLimiterMemoryStoreConfig{
				Rate:  cfg.RateLimitRPS,
				Burst: cfg.RateLimitBurst,
			}),
			DenyHandler: func(c *echo.Context, identifier string, err error) error {
				metrics.RateLimiterDrops.WithLabelValues("global").Inc()
				return middleware.ErrRateLimitExceeded.Wrap(err)
			},
		}))
	}
	e.Use(middleware.Recover())
	e.Use(appmw.CircuitBreaker(&cfg))
	e.Use(middleware.RequestID()) // sets X-Request-ID header; must run before RequestLogger
	e.Use(appmw.RequestLogger())  // wide event canonical log line
	e.Use(middleware.Secure())

	e.GET("/metrics", echoprometheus.NewHandler()) // adds route to serve gathered metrics

	e.GET("/api/v1/accounts/:id/balance", accountHandler.GetBalance)
	e.POST("/api/v1/transactions", txHandler.CreateTransaction)
	e.GET("/api/v1/transactions/:id/status", txHandler.GetTransactionStatus)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM) // start shutdown process on signal
	defer cancel()

	go metrics.StartRuntimeCollector(ctx, 5*time.Second)

	if cfg.QueueEnabled && queueClient != nil && db != nil {
		w := worker.NewWorker(db, queueClient, redisClient, txRepo)
		go w.Start(ctx, cfg.QueueWorkers)
	}

	go func() {
		collect := func() {
			if db != nil {
				pingCtx, pingCancel := context.WithTimeout(ctx, 2*time.Second)
				if err := db.PingContext(pingCtx); err != nil {
					metrics.DependencyUp.WithLabelValues("database").Set(0)
				} else {
					metrics.DependencyUp.WithLabelValues("database").Set(1)
					s := db.Stats()
					metrics.DBConnectionsActive.Set(float64(s.InUse))
					metrics.DBConnectionsIdle.Set(float64(s.Idle))
				}
				pingCancel()
			}

			if redisClient != nil {
				pingCtx, pingCancel := context.WithTimeout(ctx, 2*time.Second)
				if err := redisClient.Ping(pingCtx).Err(); err != nil {
					metrics.DependencyUp.WithLabelValues("redis").Set(0)
				} else {
					metrics.DependencyUp.WithLabelValues("redis").Set(1)
				}
				pingCancel()
			}

			if queueClient != nil {
				depth, err := queueClient.QueueDepth("transactions")
				if err != nil {
					metrics.DependencyUp.WithLabelValues("rabbitmq").Set(0)
					return
				}
				metrics.DependencyUp.WithLabelValues("rabbitmq").Set(1)
				metrics.QueueDepth.WithLabelValues("transactions").Set(float64(depth))
			}
		}

		collect()
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				collect()
			}
		}
	}()

	sc := echo.StartConfig{
		Address:         fmt.Sprintf(":%d", cfg.AppPort),
		GracefulTimeout: 5 * time.Second, // defaults to 10 seconds
	}

	if err := sc.Start(ctx, e); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
}
