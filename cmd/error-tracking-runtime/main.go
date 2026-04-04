package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/extension-sdk/eventbus"
	"github.com/movebigrocks/extension-sdk/extensionhost/infrastructure/config"
	"github.com/movebigrocks/extension-sdk/extensionhost/infrastructure/outbox"
	platformsql "github.com/movebigrocks/extension-sdk/extensionhost/infrastructure/stores/sql"
	platformservices "github.com/movebigrocks/extension-sdk/extensionhost/platform/services"
	serviceapp "github.com/movebigrocks/extension-sdk/extensionhost/service/services"
	"github.com/movebigrocks/extension-sdk/logger"
	"github.com/movebigrocks/extension-sdk/runtimehttp"
	errortrackingruntime "github.com/movebigrocks/extensions/error-tracking/runtime"
	observabilityhandlers "github.com/movebigrocks/extensions/error-tracking/runtime/handlers"
	observabilityservices "github.com/movebigrocks/extensions/error-tracking/runtime/services"
	errortrackingui "github.com/movebigrocks/extensions/error-tracking/runtimeui"
)

const packageKey = "demandops/error-tracking"

type errorTrackingRuntime struct {
	db             *platformsql.DB
	store          *platformsql.Store
	outbox         *outbox.Service
	workspace      *platformservices.WorkspaceManagementService
	user           *platformservices.UserManagementService
	extension      *platformservices.ExtensionService
	caseService    *serviceapp.CaseService
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
	tmpl, err := errortrackingui.ParseTemplates()
	if err != nil {
		log.Error("Failed to parse error-tracking templates", "error", err)
		os.Exit(1)
	}
	engine.SetHTMLTemplate(tmpl)

	registerErrorTrackingRoutes(engine, runtime, cfg.Server.APIBaseURL)
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
		return nil, fmt.Errorf("create store: %w", err)
	}

	outboxService := outbox.NewServiceWithConfig(store, eventbus.NewInMemoryBus(), log, cfg.Outbox)
	workspaceService := platformservices.NewWorkspaceManagementService(
		store.Workspaces(),
		store.Cases(),
		store.Users(),
		store.Rules(),
	)
	userService := platformservices.NewUserManagementService(
		store.Users(),
		store.Workspaces(),
	)
	extensionService := platformservices.NewExtensionService(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
	)
	caseService := serviceapp.NewCaseService(
		store.Queues(),
		store.Cases(),
		store.Workspaces(),
		outboxService,
		serviceapp.WithQueueItemStore(store.QueueItems()),
		serviceapp.WithTransactionRunner(store),
		serviceapp.WithOutboundEmailStore(store.OutboundEmails()),
		serviceapp.WithUserStore(store.Users()),
	)

	stdDB, err := db.GetSQLDB()
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("resolve sql db: %w", err)
	}

	errorStore := errortrackingruntime.NewErrorMonitoringStore(errortrackingruntime.NewSqlxDB(stdDB, db.Driver()))
	issueService := observabilityservices.NewIssueService(
		errorStore,
		errorStore,
		errorStore,
		store.Workspaces(),
		outboxService,
	)
	projectService := observabilityservices.NewProjectService(errorStore, store.Workspaces())
	errorGrouping := observabilityservices.NewErrorGroupingService(errorStore, errorStore, outboxService)
	processor := observabilityservices.NewErrorProcessorFromConfig(errorGrouping, cfg.ErrorProcessing)
	if err := processor.StartWorkers(context.Background(), cfg.ErrorProcessing.WorkerCount); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("start error processor workers: %w", err)
	}

	return &errorTrackingRuntime{
		db:             db,
		store:          store,
		outbox:         outboxService,
		workspace:      workspaceService,
		user:           userService,
		extension:      extensionService,
		caseService:    caseService,
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
	if r.store != nil {
		if err := r.store.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func registerErrorTrackingRoutes(engine *gin.Engine, runtime *errorTrackingRuntime, apiBaseURL string) {
	adminHandler := errortrackingruntime.NewErrorTrackingAdminHandler(
		runtime.workspace,
		runtime.user,
		runtime.extension,
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
		runtime.store.Cases(),
		runtime.caseService,
	)
	handler := observabilityhandlers.NewErrorTrackingCaseEventHandler(
		issueCaseService,
		runtime.store,
		logger.New().WithField("handler", "error-tracking-consumer-case-events"),
	)
	return func(ctx context.Context, data []byte) error {
		var envelope struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return err
		}
		switch strings.TrimSpace(envelope.Type) {
		case "issue.case_linked":
			return handler.HandleIssueCaseLinked(ctx, data)
		case "issue.case_unlinked":
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
