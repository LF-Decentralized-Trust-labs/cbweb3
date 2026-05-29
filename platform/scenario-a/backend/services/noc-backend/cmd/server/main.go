package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/joho/godotenv"

	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/api"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/config"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/keycloak"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/middleware"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/repository"
	"github.com/LACNetNetworks/cbweb3-platform/backend/services/noc-backend/internal/service"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := repository.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}

	var kc keycloak.Client
	if cfg.SkipAuth {
		log.Println("WARNING: NOC_SKIP_AUTH=true — Keycloak auth is DISABLED. Do not use in production.")
		kc = keycloak.NewNoOp()
	} else {
		kc, err = keycloak.New(keycloak.Config{
			BaseURL:      cfg.KeycloakURL,
			Realm:        cfg.KeycloakRealm,
			JWKSCacheTTL: cfg.JWKSCacheTTL,
		})
		if err != nil {
			log.Fatalf("keycloak: %v", err)
		}
	}

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: cfg.FrontendOrigin,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET, POST, PUT, PATCH, DELETE, OPTIONS",
	}))

	// Repositories
	spokesRepo := repository.NewSpokesRepository(db)
	agentsRepo := repository.NewAgentsRepository(db)
	componentsRepo := repository.NewComponentsRepository(db)

	// Services
	alertSvc := service.NewAlertService(db)
	relayMetricsSvc := service.NewRelayMetricsService(db)
	watchdog := service.NewWatchdogService(agentsRepo, componentsRepo, db, cfg.AgentGraceMultiplier)
	retention := service.NewRetentionWorker(db)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watchdog.Start(ctx, 15*time.Second)
	retention.Start(ctx)

	// Handlers
	spokesHandler := api.NewSpokesHandler(spokesRepo)
	keysHandler := api.NewKeysHandler(agentsRepo, spokesRepo)
	pushHandler := api.NewPushHandler(db, agentsRepo, componentsRepo, alertSvc)
	dashHandler := api.NewDashboardHandler(db, spokesRepo, agentsRepo, componentsRepo, alertSvc, relayMetricsSvc)

	// Routes — agent push (API key auth, no Keycloak)
	pushHandler.Register(app)

	// Routes — admin (Keycloak JWT + noc-admin role required)
	admin := app.Group("/api/v1/admin",
		middleware.RequireAuth(kc),
		middleware.RequireRole("noc-admin"),
	)
	spokesHandler.Register(admin)
	keysHandler.Register(admin)

	// Routes — NOC portal read (Keycloak JWT, any NOC role)
	portal := app.Group("/api/v1",
		middleware.RequireAuth(kc),
	)
	dashHandler.Register(portal)

	// Health probe
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-quit
		log.Println("shutting down...")
		_ = app.Shutdown()
	}()

	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("noc-backend listening on %s", addr)
	if err := app.Listen(addr); err != nil {
		log.Fatalf("server: %v", err)
	}
}
