package main

import (
	"log"

	relayer "github.com/GoPolymarket/builder-relayer-go-client"
	"github.com/GoPolymarket/polygate/internal/config"
	"github.com/GoPolymarket/polygate/internal/handler"
	"github.com/GoPolymarket/polygate/internal/middleware"
	"github.com/GoPolymarket/polygate/internal/repository"
	"github.com/GoPolymarket/polygate/internal/service"
	"github.com/gin-gonic/gin"
)

func main() {
	// 1. Load Configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 2. Initialize Core Services
	// TenantManager holds the "database" of users and their cached SDK clients
	tenantManager := service.NewTenantManager(cfg, nil)
	idempotencyStore := middleware.NewInMemIdempotencyStore()
	
	// Risk Engine (Auto-detect DB)
	var riskRepo service.UsageRepo
	db, err := repository.NewDB(cfg)
	if err == nil {
		log.Println("✅ Connected to PostgreSQL, using persistent storage.")
		riskRepo = repository.NewPostgresRiskRepo(db)
		// TODO: Also upgrade TenantManager to use DB (need refactor TenantManager to accept repo)
	} else {
		log.Printf("⚠️  DB connection failed (%v), falling back to In-Memory storage.", err)
		riskRepo = service.NewRiskUsageStore()
	}
	
	riskEngine := service.NewRiskEngine(riskRepo)
	
	// Audit Service
	auditSvc, err := service.NewAuditService("./logs", nil)
	if err != nil {
		log.Fatalf("Failed to initialize audit service: %v", err)
	}
	defer auditSvc.Close()

	// GatewayService logic now depends on TenantManager
	gatewaySvc, err := service.NewGatewayService(cfg, tenantManager, riskEngine)
	if err != nil {
		log.Fatalf("Failed to initialize gateway service: %v", err)
	}

	// Relayer Client - Create builder config for relayer operations
	builderConfig := &relayer.BuilderConfig{
		Local: &relayer.BuilderCredentials{
			Key:        cfg.Builder.ApiKey,
			Secret:     cfg.Builder.ApiSecret,
			Passphrase: cfg.Builder.ApiPassphrase,
		},
	}

	// Account Service (Relayer/Safe deployment)
	accountSvc := service.NewAccountService(tenantManager, nil, builderConfig)

	// 3. Initialize Handlers
	orderHandler := handler.NewOrderHandler(gatewaySvc)
	accountHandler := handler.NewAccountHandler(accountSvc)

	// 4. Setup Router
	r := gin.Default()
	
	// Global Middleware
	r.Use(middleware.AuditMiddleware(auditSvc))

	// Health Check (Public)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "polygate"})
	})

	// API V1 Routes (Protected)
	v1 := r.Group("/v1")
	
	// Apply Auth Middleware to V1 routes
	// This requires X-Gateway-Key header and injects the Tenant into context
	v1.Use(middleware.AuthMiddleware(cfg, tenantManager))
	v1.Use(middleware.RateLimitMiddleware(tenantManager))
	v1.Use(middleware.IdempotencyMiddleware(idempotencyStore))
	{
		// Order Management
		v1.POST("/orders", orderHandler.PlaceOrder)
		v1.DELETE("/orders/:id", orderHandler.CancelOrder)
		v1.DELETE("/orders", orderHandler.CancelAll)
		
		// Account Management
		v1.GET("/account/proxy", accountHandler.GetProxy)
		v1.POST("/account/proxy", accountHandler.DeployProxy)
	}

	// 5. Start Server
	log.Printf("🚀 PolyGate started on port %s", cfg.Server.Port)
	if err := r.Run(":" + cfg.Server.Port); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}