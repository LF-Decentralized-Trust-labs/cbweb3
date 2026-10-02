// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
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
			Audience:     cfg.KeycloakAudience,
			JWKSCacheTTL: cfg.JWKSCacheTTL,
			// Needed by the password grant the login performs. noc-portal is a public
			// client in the realms the toolkit provisions, so the secret is normally
			// empty and is then omitted from the form rather than sent blank.
			ClientID:     cfg.KeycloakClientID,
			ClientSecret: cfg.KeycloakClientSecret,
		})
		if err != nil {
			log.Fatalf("keycloak: %v", err)
		}
		issuer := strings.TrimRight(cfg.KeycloakURL, "/") + "/realms/" + cfg.KeycloakRealm
		if cfg.KeycloakAudience == "" {
			log.Printf("keycloak: token validation — issuer=%q, audience enforcement DISABLED (KEYCLOAK_AUDIENCE unset)", issuer)
		} else {
			log.Printf("keycloak: token validation — issuer=%q, audience=%q", issuer, cfg.KeycloakAudience)
		}
	}

	app := fiber.New(fiber.Config{
		// Connection timeouts (finding R2-LOW). Without ReadTimeout a connection can open,
		// dribble its headers and hold a server slot indefinitely — the slowloris shape, and
		// nothing in the handler chain bounds it because it happens before any handler runs.
		// IdleTimeout does the same for keep-alive connections that go quiet. Safe to set
		// firmly here: this service exposes request/response dashboard reads only — no
		// text/event-stream or websocket endpoint exists in either scenario's Go backends,
		// so there is no long-lived stream for WriteTimeout to cut.
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{"error": err.Error()})
		},
	})

	// The CSRF secret keys the HMAC binding each token to its session. Warned about loudly
	// rather than generated quietly: a per-process secret works in a single container and
	// then refuses every token the moment there are two replicas, or the moment one
	// restarts — a failure that looks like a browser bug, not a configuration gap.
	csrfSecret := []byte(cfg.CSRFSecret)
	if len(csrfSecret) == 0 {
		log.Println("WARNING: CSRF_SECRET is not set — generating a per-process secret. " +
			"Every CSRF token becomes invalid on restart, and replicas will reject each other's.")
		csrfSecret = make([]byte, 32)
		if _, rErr := rand.Read(csrfSecret); rErr != nil {
			log.Fatalf("csrf: could not generate a secret: %v", rErr)
		}
	}
	if !cfg.CookieSecure {
		log.Println("WARNING: COOKIE_SECURE=false — auth cookies are sent over plain HTTP. " +
			"Local development only.")
	}

	app.Use(recover.New())
	app.Use(logger.New())
	// AllowCredentials is mandatory now that the session is a cookie: a browser does NOT
	// attach cookies to a cross-origin request unless the response says so, and the NOC
	// portal is served from a different origin than this API. Without it the portal logs in,
	// receives its cookies, and is then anonymous on every subsequent request — with no
	// error anywhere to explain it.
	//
	// It also forbids AllowOrigins "*": the browser refuses credentials against a wildcard,
	// so FrontendOrigin has to name the portal explicitly.
	//
	// X-XSRF-TOKEN joins the allowed headers because the double-submit check reads it.
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.FrontendOrigin,
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization, X-XSRF-TOKEN",
		AllowMethods:     "GET, POST, PUT, PATCH, DELETE, OPTIONS",
		AllowCredentials: true,
	}))

	// Repositories
	spokesRepo := repository.NewSpokesRepository(db)
	agentsRepo := repository.NewAgentsRepository(db)
	componentsRepo := repository.NewComponentsRepository(db)

	// Services
	alertSvc := service.NewAlertService(db)
	relayMetricsSvc := service.NewRelayMetricsService(db)
	watchdog := service.NewWatchdogService(agentsRepo, componentsRepo, alertSvc, db, cfg.AgentGraceMultiplier)
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

	// Routes and the authorization each group requires — see routes.go, which is
	// separate so routes_test.go can assert the wiring rather than only the middleware.
	mountRoutes(app, kc, nocRoutes{
		Push:   pushHandler,
		Admin:  []routeRegistrar{spokesHandler, keysHandler},
		Portal: []routeRegistrar{dashHandler},
		Auth:   api.NewAuthHandler(kc, cfg.CookieSecure, csrfSecret),
	}, csrfSecret)

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
