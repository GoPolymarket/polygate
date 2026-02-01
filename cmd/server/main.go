package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	relayer "github.com/GoPolymarket/go-builder-relayer-client"
	"github.com/GoPolymarket/polygate/internal/config"
	"github.com/GoPolymarket/polygate/internal/handler"
	"github.com/GoPolymarket/polygate/internal/market"
	"github.com/GoPolymarket/polygate/internal/middleware"
	"github.com/GoPolymarket/polygate/internal/pkg/logger"
	"github.com/GoPolymarket/polygate/internal/repository"
	"github.com/GoPolymarket/polygate/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// 0. Initialize Logger
	logger.Init("info")

	// 1. Load Configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Initialize Persistence
	// Risk Persistence (Redis > Memory)
	var riskRepo service.UsageRepo
	if cfg.Redis.Addr != "" {
		redisClient, err := repository.NewRedisClient(cfg)
		if err == nil {
			logger.Info("✅ Connected to Redis")
			riskRepo = redisClient
		} else {
			logger.Error("⚠️ Failed to connect to Redis, falling back to memory", "error", err)
		}
	}
	if riskRepo == nil {
		riskRepo = service.NewRiskUsageStore()
	}

	// Audit Persistence (Postgres > Local File)
	var auditRepo service.AuditRepo
	if cfg.Database.DSN != "" {
		db, err := repository.NewDB(cfg)
		if err == nil {
			logger.Info("✅ Connected to PostgreSQL")
			auditRepo = repository.NewPostgresAuditRepo(db)
		} else {
			logger.Error("⚠️ Failed to connect to DB, audit logs will be file-only", "error", err)
		}
	}

	// 3. Initialize Core Services
	tenantManager := service.NewTenantManager(cfg, nil)
	idempotencyStore := middleware.NewInMemIdempotencyStore()
	
	// Market Data Service
	marketSvc := market.NewMarketService()
	marketSvc.Start()
	
	// User Execution Stream
	var userStream *market.UserStream
	if cfg.Polymarket.ApiKey != "" {
		userStream = market.NewUserStream(cfg.Polymarket.ApiKey, cfg.Polymarket.ApiSecret, cfg.Polymarket.ApiPassphrase)
		userStream.Start()
	}
	
	riskEngine := service.NewRiskEngine(riskRepo, marketSvc)
	
	auditSvc, err := service.NewAuditService("./logs", auditRepo)
	if err != nil {
		log.Fatalf("Failed to initialize audit service: %v", err)
	}

	gatewaySvc, err := service.NewGatewayService(cfg, tenantManager, riskEngine, marketSvc, userStream)
	if err != nil {
		log.Fatalf("Failed to initialize gateway service: %v", err)
	}

	builderConfig := &relayer.BuilderConfig{
		Local: &relayer.BuilderCredentials{
			Key:        cfg.Builder.ApiKey,
			Secret:     cfg.Builder.ApiSecret,
			Passphrase: cfg.Builder.ApiPassphrase,
		},
	}

	accountSvc := service.NewAccountService(tenantManager, nil, builderConfig)

	// 4. Initialize Handlers
	orderHandler := handler.NewOrderHandler(gatewaySvc)
	accountHandler := handler.NewAccountHandler(accountSvc)

	// 5. Setup Router
	r := gin.Default()
	
	// Global Middleware
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.MetricsMiddleware()) // New Metrics Middleware
	r.Use(middleware.AuditMiddleware(auditSvc))

	// Health Check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "polygate"})
	})

	// Metrics Endpoint
	if cfg.Metrics.Enabled {
		r.GET(cfg.Metrics.Path, gin.WrapH(promhttp.Handler()))
	}

	// API V1 Routes
	v1 := r.Group("/v1")
	v1.Use(middleware.AuthMiddleware(cfg, tenantManager))
	v1.Use(middleware.RateLimitMiddleware(tenantManager))
	v1.Use(middleware.IdempotencyMiddleware(idempotencyStore))
	{
		v1.POST("/orders", orderHandler.PlaceOrder)
		v1.DELETE("/orders/:id", orderHandler.CancelOrder)
		v1.DELETE("/orders", orderHandler.CancelAll)
		v1.DELETE("/panic", orderHandler.Panic)
		v1.GET("/fills", orderHandler.GetFills)
		v1.GET("/markets/:id/book", orderHandler.GetOrderbook)
		v1.GET("/account/proxy", accountHandler.GetProxy)
		v1.POST("/account/proxy", accountHandler.DeployProxy)
	}

	// 6. Start Server with Graceful Shutdown
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: r,
	}

	go func() {
		logger.Info("🚀 PolyGate started", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server listen failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("🛑 Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	marketSvc.Stop()
	auditSvc.Close()
	
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown: ", err)
	}

	logger.Info("Server exiting")
}
