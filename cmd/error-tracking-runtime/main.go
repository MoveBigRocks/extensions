package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/extension-sdk/eventbus"
	"github.com/movebigrocks/extension-sdk/extdb"
	"github.com/movebigrocks/extension-sdk/logger"
	"github.com/movebigrocks/extension-sdk/runtimehttp"
	errortrackingruntime "github.com/movebigrocks/extensions/error-tracking/runtime"
	"github.com/movebigrocks/extensions/error-tracking/runtime/config"
	observabilityhandlers "github.com/movebigrocks/extensions/error-tracking/runtime/handlers"
	"github.com/movebigrocks/extensions/error-tracking/runtime/hostclient"
	observabilityservices "github.com/movebigrocks/extensions/error-tracking/runtime/services"
	errortrackingui "github.com/movebigrocks/extensions/error-tracking/runtimeui"
)

const packageKey = "demandops/error-tracking"

type errorTrackingRuntime struct {
	db             *extdb.DB
	issueService   *observabilityservices.IssueService
	projectService *observabilityservices.ProjectService
	processor      *observabilityservices.ErrorProcessor
	errorStore     *errortrackingruntime.ErrorMonitoringStore
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}
	log := logger.New().WithField("service", "error-tracking-runtime")

	runtime, err := newErrorTrackingRuntime(cfg, log)
	if err != nil {
		log.Error("Failed to initialize error-tracking runtime container", "error", err)
		os.Exit(1)
	}
	defer func() {
		if stopErr := runtime.Close(); stopErr != nil {
			log.Warn("Failed to stop error-tracking runtime container", "error", stopErr)
		}
	}()

	engine := runtimehttp.DefaultEngine()
	engine.Use(hostclient.Middleware())
	// The admin role applies only to the extension-owned schema. Core data is
	// reached through the host client, where instance scope and permissions are
	// enforced independently for every operation.
	engine.Use(runtimehttp.AdminContext(runtime.db))
	tmpl, err := errortrackingui.ParseTemplates()
	if err != nil {
		log.Error("Failed to parse error-tracking templates", "error", err)
		os.Exit(1)
	}
	engine.SetHTMLTemplate(tmpl)

	registerErrorTrackingRoutes(engine, runtime, cfg.APIBaseURL)
	runtimehttp.RegisterInternalRoutes(engine, map[string]func(context.Context, []byte) error{
		"error-tracking.consumer.errors":       newErrorConsumer(runtime),
		"error-tracking.consumer.issue-events": newIssueConsumer(runtime),
		"error-tracking.consumer.case-events":  newCaseConsumer(runtime),
	}, nil)

	log.Info("Starting error-tracking extension runtime", "package_key", packageKey)
	if err := runtimehttp.ListenAndServeUnixSocket(engine, packageKey); err != nil && err != http.ErrServerClosed {
		log.Error("Error-tracking runtime stopped", "error", err)
		os.Exit(1)
	}
}

func newErrorTrackingRuntime(cfg *config.Config, log *logger.Logger) (*errorTrackingRuntime, error) {
	db, err := extdb.Open(extdb.Config{
		DSN:             cfg.DatabaseDSN,
		MaxOpenConns:    cfg.DatabasePool.MaxOpenConns,
		MaxIdleConns:    cfg.DatabasePool.MaxIdleConns,
		ConnMaxLifetime: cfg.DatabasePool.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.DatabasePool.ConnMaxIdleTime,
	})
	if err != nil {
		return nil, fmt.Errorf("create database: %w", err)
	}

	errorStore := errortrackingruntime.NewErrorMonitoringStore(db)
	publisher := hostclient.Publisher{Provider: hostclient.FromContext}
	issueService := observabilityservices.NewIssueService(
		errorStore,
		errorStore,
		errorStore,
		publisher,
	)
	projectService := observabilityservices.NewProjectService(errorStore)
	errorGrouping := observabilityservices.NewErrorGroupingService(errorStore, errorStore, publisher)
	processor := observabilityservices.NewErrorProcessorFromConfig(errorGrouping, cfg.ErrorProcessing, db)
	if err := processor.StartWorkers(context.Background(), cfg.ErrorProcessing.WorkerCount); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("start error processor workers: %w", err)
	}

	return &errorTrackingRuntime{
		db:             db,
		issueService:   issueService,
		projectService: projectService,
		processor:      processor,
		errorStore:     errorStore,
	}, nil
}

func (r *errorTrackingRuntime) Close() error {
	if r == nil {
		return nil
	}
	var firstErr error
	if r.processor != nil {
		if err := r.processor.StopWorkers(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if r.db != nil {
		if err := r.db.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func registerErrorTrackingRoutes(engine *gin.Engine, runtime *errorTrackingRuntime, apiBaseURL string) {
	adminHandler := errortrackingruntime.NewErrorTrackingAdminHandler(
		hostclient.FromContext,
		runtime.issueService,
		runtime.projectService,
		apiBaseURL,
	)

	sentryIngestHandler := observabilityhandlers.NewSentryIngestHandler(
		runtime.projectService,
		runtime.errorStore,
		runtime.processor,
		logger.New().WithField("handler", "error-tracking-ingest"),
	)

	engine.GET("/extensions/error-tracking/applications", adminHandler.ShowApplications)
	engine.HEAD("/extensions/error-tracking/applications", adminHandler.ShowApplications)
	engine.GET("/extensions/error-tracking/applications/new", adminHandler.ShowApplicationDetail)
	engine.HEAD("/extensions/error-tracking/applications/new", adminHandler.ShowApplicationDetail)
	engine.GET("/extensions/error-tracking/applications/:id", adminHandler.ShowApplicationDetail)
	engine.HEAD("/extensions/error-tracking/applications/:id", adminHandler.ShowApplicationDetail)
	engine.POST("/extensions/error-tracking/applications", adminHandler.CreateApplication)
	engine.PUT("/extensions/error-tracking/applications/:id", adminHandler.UpdateApplication)
	engine.DELETE("/extensions/error-tracking/applications/:id", adminHandler.DeleteApplication)

	engine.GET("/extensions/error-tracking/issues", adminHandler.ShowIssues)
	engine.HEAD("/extensions/error-tracking/issues", adminHandler.ShowIssues)
	engine.GET("/extensions/error-tracking/issues/:id", adminHandler.ShowIssueDetail)
	engine.HEAD("/extensions/error-tracking/issues/:id", adminHandler.ShowIssueDetail)

	engine.POST("/api/envelope", sentryIngestHandler.HandleEnvelope)
	engine.POST("/api/:projectNumber/envelope", sentryIngestHandler.HandleEnvelopeWithProject)
	engine.POST("/1/envelope", sentryIngestHandler.HandleEnvelope)

	engine.GET("/extensions/error-tracking/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"message": "error tracking runtime ready",
		})
	})
	engine.HEAD("/extensions/error-tracking/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
}

func newErrorConsumer(runtime *errorTrackingRuntime) func(context.Context, []byte) error {
	handler := observabilityhandlers.NewErrorEventHandler(
		runtime.processor,
		logger.New().WithField("handler", "error-tracking-consumer-errors"),
	)
	return handler.HandleErrorEvent
}

func newIssueConsumer(runtime *errorTrackingRuntime) func(context.Context, []byte) error {
	handler := observabilityhandlers.NewIssueEventHandler(
		runtime.issueService,
		logger.New().WithField("handler", "error-tracking-consumer-issues"),
	)
	return func(ctx context.Context, data []byte) error {
		switch eventType := strings.TrimSpace(eventbus.ParseEventType(data)); eventType {
		case "", "issue.created":
			return handler.HandleIssueCreated(ctx, data)
		case "issue.updated":
			return handler.HandleIssueUpdated(ctx, data)
		case "issue.resolved":
			return handler.HandleIssueResolved(ctx, data)
		default:
			return nil
		}
	}
}

func newCaseConsumer(runtime *errorTrackingRuntime) func(context.Context, []byte) error {
	issueCaseService := observabilityservices.NewIssueCaseService(
		hostclient.FromContext,
	)
	handler := observabilityhandlers.NewErrorTrackingCaseEventHandler(
		issueCaseService,
		logger.New().WithField("handler", "error-tracking-consumer-case-events"),
	)
	return func(ctx context.Context, data []byte) error {
		switch strings.TrimSpace(eventbus.ParseEventType(data)) {
		case "issue_case.linked":
			return handler.HandleIssueCaseLinked(ctx, data)
		case "issue_case.unlinked":
			return handler.HandleIssueCaseUnlinked(ctx, data)
		case "case.created_for_contact":
			return handler.HandleCaseCreatedForContact(ctx, data)
		case "cases.bulk_resolved":
			return handler.HandleCasesBulkResolved(ctx, data)
		default:
			return nil
		}
	}
}
