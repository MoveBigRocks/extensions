package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/extension-sdk/extensionhost/infrastructure/config"
	"github.com/movebigrocks/extension-sdk/extensionhost/shared/geoip"
	"github.com/movebigrocks/extension-sdk/logger"
	"github.com/movebigrocks/extension-sdk/runtimehttp"
	sqlstore "github.com/movebigrocks/extensions/web-analytics/runtime"
	analyticsdomain "github.com/movebigrocks/extensions/web-analytics/runtime/domain"
	analyticshandlers "github.com/movebigrocks/extensions/web-analytics/runtime/handlers"
	analyticsservices "github.com/movebigrocks/extensions/web-analytics/runtime/services"
	webanalyticsui "github.com/movebigrocks/extensions/web-analytics/runtimeui"
)

const packageKey = "demandops/web-analytics"

type analyticsRuntime struct {
	ingest *analyticsservices.IngestService
	query  *analyticsservices.QueryService
	store  *sqlstore.AnalyticsStore
	db     *sqlstore.AnalyticsDB
	log    *logger.Logger
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New().WithField("service", "web-analytics-runtime")
	runtime, err := newAnalyticsRuntime(cfg, log)
	if err != nil {
		log.Error("Failed to initialize web analytics runtime", "error", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := runtime.db.Close(); closeErr != nil {
			log.Warn("Failed to close analytics runtime database", "error", closeErr)
		}
	}()

	engine := runtimehttp.DefaultEngine()
	tmpl, err := webanalyticsui.ParseTemplates()
	if err != nil {
		log.Error("Failed to parse analytics templates", "error", err)
		os.Exit(1)
	}
	engine.SetHTMLTemplate(tmpl)

	scriptContent, err := webanalyticsui.Assets.ReadFile("assets/analytics.js")
	if err != nil {
		log.Error("Failed to load analytics runtime asset", "error", err)
		os.Exit(1)
	}

	registerAnalyticsRoutes(engine, cfg, runtime, scriptContent)
	runtimehttp.RegisterInternalRoutes(engine, nil, map[string]func(context.Context) error{
		"analytics.job.maintenance": runtime.runMaintenance,
	})

	log.Info("Starting web analytics extension runtime", "package_key", packageKey)
	if err := runtimehttp.ListenAndServeUnixSocket(engine, packageKey); err != nil && err != http.ErrServerClosed {
		log.Error("Analytics runtime stopped", "error", err)
		os.Exit(1)
	}
}

func newAnalyticsRuntime(cfg *config.Config, log *logger.Logger) (*analyticsRuntime, error) {
	geo := geoip.NewNoopService()
	if cfg != nil && cfg.GeoIPDBPath != "" {
		if svc, err := geoip.NewMaxMindService(cfg.GeoIPDBPath); err == nil {
			geo = svc
		} else if log != nil {
			log.Warn("Failed to load GeoIP database for analytics runtime", "path", cfg.GeoIPDBPath, "error", err)
		}
	}

	db, err := sqlstore.NewAnalyticsDB(cfg.Database.EffectiveDSN())
	if err != nil {
		return nil, err
	}
	store := sqlstore.NewAnalyticsStore(db)
	return &analyticsRuntime{
		ingest: analyticsservices.NewIngestService(store, geo, log),
		query:  analyticsservices.NewQueryService(store),
		store:  store,
		db:     db,
		log:    log,
	}, nil
}

func registerAnalyticsRoutes(engine *gin.Engine, cfg *config.Config, runtime *analyticsRuntime, scriptContent []byte) {
	adminHandler := analyticshandlers.NewAnalyticsAdminHandler()
	apiHandler := analyticshandlers.NewAnalyticsExtensionAPIHandler(runtime.query, cfg.Server.APIBaseURL)
	ingestHandler := analyticshandlers.NewAnalyticsIngestHandler(runtime.ingest, logger.New().WithField("handler", "analytics-ingest"))
	scriptHandler := analyticshandlers.NewAnalyticsScriptHandlerWithContent(scriptContent)

	engine.GET("/js/analytics.js", scriptHandler.ServeScript)
	engine.HEAD("/js/analytics.js", scriptHandler.ServeScript)
	engine.POST("/api/analytics/event", ingestHandler.HandleEvent)

	engine.GET("/extensions/web-analytics", adminHandler.ShowAnalyticsProperties)
	engine.HEAD("/extensions/web-analytics", adminHandler.ShowAnalyticsProperties)
	engine.GET("/extensions/web-analytics/:id", adminHandler.ShowPropertyDashboard)
	engine.HEAD("/extensions/web-analytics/:id", adminHandler.ShowPropertyDashboard)
	engine.GET("/extensions/web-analytics/:id/setup", adminHandler.ShowPropertySetup)
	engine.HEAD("/extensions/web-analytics/:id/setup", adminHandler.ShowPropertySetup)
	engine.GET("/extensions/web-analytics/:id/settings", adminHandler.ShowPropertySettings)
	engine.HEAD("/extensions/web-analytics/:id/settings", adminHandler.ShowPropertySettings)

	engine.GET("/extensions/web-analytics/api/properties", apiHandler.ListProperties)
	engine.POST("/extensions/web-analytics/api/properties", apiHandler.CreateProperty)
	engine.GET("/extensions/web-analytics/api/properties/:id", apiHandler.GetProperty)
	engine.PATCH("/extensions/web-analytics/api/properties/:id", apiHandler.UpdateProperty)
	engine.DELETE("/extensions/web-analytics/api/properties/:id", apiHandler.DeleteProperty)
	engine.POST("/extensions/web-analytics/api/properties/:id/reset", apiHandler.ResetProperty)
	engine.GET("/extensions/web-analytics/api/properties/:id/current-visitors", apiHandler.CurrentVisitors)
	engine.GET("/extensions/web-analytics/api/properties/:id/verify", apiHandler.VerifyInstallation)
	engine.GET("/extensions/web-analytics/api/properties/:id/metrics", apiHandler.Metrics)
	engine.GET("/extensions/web-analytics/api/properties/:id/timeseries", apiHandler.TimeSeries)
	engine.GET("/extensions/web-analytics/api/properties/:id/breakdown", apiHandler.Breakdown)
	engine.GET("/extensions/web-analytics/api/properties/:id/goals", apiHandler.ListGoals)
	engine.GET("/extensions/web-analytics/api/properties/:id/goal-results", apiHandler.GoalResults)
	engine.POST("/extensions/web-analytics/api/properties/:id/goals", apiHandler.CreateGoal)
	engine.DELETE("/extensions/web-analytics/api/goals/:goalID", apiHandler.DeleteGoal)
	engine.GET("/extensions/web-analytics/api/properties/:id/hostname-rules", apiHandler.ListHostnameRules)
	engine.POST("/extensions/web-analytics/api/properties/:id/hostname-rules", apiHandler.CreateHostnameRule)
	engine.DELETE("/extensions/web-analytics/api/hostname-rules/:ruleID", apiHandler.DeleteHostnameRule)

	engine.GET("/extensions/web-analytics/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"message": "analytics runtime ready",
		})
	})
	engine.HEAD("/extensions/web-analytics/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
}

func (r *analyticsRuntime) runMaintenance(ctx context.Context) error {
	salts, err := r.store.GetCurrentSalts(ctx)
	if err != nil {
		return err
	}
	if len(salts) == 0 {
		salt, err := analyticsdomain.NewSalt()
		if err != nil {
			return err
		}
		if err := r.store.InsertSalt(ctx, salt); err != nil {
			return err
		}
	}

	r.ingest.RefreshCaches(ctx)

	needsRotation := len(salts) == 0
	if !needsRotation && len(salts) > 0 {
		needsRotation = time.Since(salts[0].CreatedAt) > 24*time.Hour
	}
	if needsRotation {
		salt, err := analyticsdomain.NewSalt()
		if err != nil {
			return err
		}
		if err := r.store.InsertSalt(ctx, salt); err != nil {
			return err
		}
	}

	return r.store.DeleteSaltsOlderThan(ctx, time.Now().UTC().Add(-48*time.Hour))
}
