package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"clubops_portal/internal/handlers"
	"clubops_portal/internal/middleware"
	"clubops_portal/internal/services"
	"clubops_portal/internal/store"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"
)

func main() {
	dbPath := os.Getenv("APP_DB_PATH")
	if dbPath == "" {
		dbPath = "./fullstack.db"
	}
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	logEvent("startup", "info", "opening sqlite at %s", dbPath)
	st, err := store.NewSQLiteStore(dbPath)
	if err != nil {
		logEvent("startup", "fatal", "open db: %v", err)
		log.Fatalf("open db: %v", err)
	}
	defer st.Close()

	if err := st.AutoMigrate(); err != nil {
		logEvent("startup", "fatal", "migrate db: %v", err)
		log.Fatalf("migrate db: %v", err)
	}
	if err := st.SeedDefaults(); err != nil {
		logEvent("startup", "fatal", "seed defaults: %v", err)
		log.Fatalf("seed defaults: %v", err)
	}

	authSvc := services.NewAuthService(st, 30*time.Minute, 5, 15*time.Minute)
	financeSvc := services.NewFinanceService(st)
	creditSvc := services.NewCreditService(st)
	reviewSvc := services.NewReviewService(st, "./static/uploads")
	mdmSvc := services.NewMDMService(st)
	auditSvc := services.NewAuditService(st)
	flagSvc := services.NewFlagService(st)
	cryptoSvc, err := services.NewCryptoService()
	if err != nil {
		logEvent("startup", "fatal", "init crypto service: %v", err)
		log.Fatalf("init crypto service: %v", err)
	}

	logEvent("worker", "info", "starting threshold and audit workers")
	go financeSvc.StartThresholdWorker(60 * time.Second)
	go auditSvc.StartRetentionWorker(24 * time.Hour)

	engine := html.New("./views", ".html")
	app := fiber.New(fiber.Config{Views: engine, BodyLimit: 15 * 1024 * 1024})
	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format:     "category=http_request level=info method=${method} path=${path} status=${status} latency=${latency} ip=${ip}\n",
		TimeFormat: time.RFC3339,
	}))
	app.Use(middleware.CSRFProtection())
	app.Use(middleware.AttachCurrentUser(authSvc))
	app.Use(middleware.AuditTrail(auditSvc, st))

	h := handlers.NewHandler(st, authSvc, financeSvc, creditSvc, reviewSvc, mdmSvc, cryptoSvc, flagSvc)
	h.RegisterRoutes(app)

	listenAddr := ":" + port
	logEvent("startup", "info", "ClubOps server listening on %s", listenAddr)
	if err := app.Listen(listenAddr); err != nil {
		logEvent("startup", "fatal", "listen: %v", err)
		log.Fatalf("listen: %v", err)
	}
}

func logEvent(category, level, format string, args ...any) {
	log.Printf("category=%s level=%s %s", category, level, fmt.Sprintf(format, args...))
}
