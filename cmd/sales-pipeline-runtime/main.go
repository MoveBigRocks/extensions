package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/platform/extensions/common/runtimehttp"
	salespipeline "github.com/movebigrocks/platform/extensions/sales-pipeline/runtime"
	salespipelineui "github.com/movebigrocks/platform/extensions/sales-pipeline/runtimeui"
	"github.com/movebigrocks/platform/internal/infrastructure/config"
	platformsql "github.com/movebigrocks/platform/internal/infrastructure/stores/sql"
	"github.com/movebigrocks/platform/pkg/logger"
)

const packageKey = "demandops/sales-pipeline"

type salesPipelineRuntime struct {
	store    *platformsql.Store
	pipeline *salespipeline.Store
	handler  *salespipeline.Handler
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New().WithField("service", "sales-pipeline-runtime")
	runtime, err := newSalesPipelineRuntime(cfg)
	if err != nil {
		log.Error("Failed to initialize sales pipeline runtime", "error", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := runtime.Close(); closeErr != nil {
			log.Warn("Failed to close sales pipeline runtime", "error", closeErr)
		}
	}()

	engine := runtimehttp.DefaultEngine()
	tmpl, err := salespipelineui.ParseTemplates()
	if err != nil {
		log.Error("Failed to parse sales pipeline templates", "error", err)
		os.Exit(1)
	}
	engine.SetHTMLTemplate(tmpl)
	registerRoutes(engine, runtime.handler)

	log.Info("Starting sales pipeline runtime", "package_key", packageKey)
	if err := runtimehttp.ListenAndServeUnixSocket(engine, packageKey); err != nil && err != http.ErrServerClosed {
		log.Error("Sales pipeline runtime stopped", "error", err)
		os.Exit(1)
	}
}

func newSalesPipelineRuntime(cfg *config.Config) (*salesPipelineRuntime, error) {
	db, err := platformsql.NewDBWithConfig(platformsql.DBConfig{
		DSN:             cfg.Database.EffectiveDSN(),
		MaxOpenConns:    cfg.DatabasePool.MaxOpenConns,
		MaxIdleConns:    cfg.DatabasePool.MaxIdleConns,
		ConnMaxLifetime: cfg.DatabasePool.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.DatabasePool.ConnMaxIdleTime,
	})
	if err != nil {
		return nil, fmt.Errorf("create database: %w", err)
	}

	store, err := platformsql.NewStore(db)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create platform store: %w", err)
	}

	pipelineStore, err := salespipeline.NewStore(store.SqlxDB())
	if err != nil {
		_ = store.Close()
		return nil, err
	}

	handler := salespipeline.NewHandler(pipelineStore, store.Extensions())
	return &salesPipelineRuntime{
		store:    store,
		pipeline: pipelineStore,
		handler:  handler,
	}, nil
}

func (r *salesPipelineRuntime) Close() error {
	if r == nil || r.store == nil {
		return nil
	}
	return r.store.Close()
}

func registerRoutes(engine *gin.Engine, handler *salespipeline.Handler) {
	engine.GET("/extensions/sales-pipeline", handler.ShowDashboard)
	engine.HEAD("/extensions/sales-pipeline", handler.ShowDashboard)
	engine.GET("/extensions/sales-pipeline/api/board", handler.GetBoard)
	engine.POST("/extensions/sales-pipeline/api/deals", handler.CreateDeal)
	engine.PATCH("/extensions/sales-pipeline/api/deals/:id/stage", handler.MoveDeal)
	engine.GET("/extensions/sales-pipeline/health", handler.Health)
	engine.HEAD("/extensions/sales-pipeline/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
}
