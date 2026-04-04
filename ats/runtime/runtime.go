package atsruntime

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"

	artifactservices "github.com/movebigrocks/extension-sdk/artifacts/services"
	"github.com/movebigrocks/extension-sdk/eventbus"
	automationservices "github.com/movebigrocks/extension-sdk/extensionhost/automation/services"
	"github.com/movebigrocks/extension-sdk/extensionhost/infrastructure/config"
	"github.com/movebigrocks/extension-sdk/extensionhost/infrastructure/outbox"
	platformsql "github.com/movebigrocks/extension-sdk/extensionhost/infrastructure/stores/sql"
	platformservices "github.com/movebigrocks/extension-sdk/extensionhost/platform/services"
	serviceapp "github.com/movebigrocks/extension-sdk/extensionhost/service/services"
	"github.com/movebigrocks/extension-sdk/logger"
)

type Runtime struct {
	Store       *platformsql.Store
	ATSStore    *Store
	Service     *Service
	Handler     *Handler
	RulesEngine *automationservices.RulesEngine
}

type RuntimeOption func(*runtimeOptions)

type runtimeOptions struct {
	extensionOptions []platformservices.ExtensionServiceOption
	attachment       attachmentUploader
}

func WithManagedArtifactPath(path string) RuntimeOption {
	return func(options *runtimeOptions) {
		if options == nil || strings.TrimSpace(path) == "" {
			return
		}
		options.extensionOptions = append(
			options.extensionOptions,
			platformservices.WithExtensionArtifactService(artifactservices.NewGitService(strings.TrimSpace(path))),
		)
	}
}

func WithAttachmentService(service attachmentUploader) RuntimeOption {
	return func(options *runtimeOptions) {
		if options == nil || service == nil {
			return
		}
		options.attachment = service
	}
}

func NewRuntimeFromConfig(cfg *config.Config) (*Runtime, error) {
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

	options := make([]RuntimeOption, 0, 2)
	if strings.TrimSpace(cfg.Storage.Artifacts.Path) != "" {
		options = append(options, WithManagedArtifactPath(cfg.Storage.Artifacts.Path))
	}
	if strings.TrimSpace(cfg.Storage.Attachments.Bucket) != "" {
		attachmentService, err := serviceapp.NewAttachmentService(serviceapp.AttachmentServiceConfig{
			S3Endpoint:    cfg.Storage.Operational.Endpoint,
			S3Region:      cfg.Storage.Operational.Region,
			S3Bucket:      cfg.Storage.Attachments.Bucket,
			S3AccessKey:   cfg.Storage.Operational.AccessKey,
			S3SecretKey:   cfg.Storage.Operational.SecretKey,
			ClamAVAddr:    cfg.Integrations.ClamAVAddr,
			ClamAVTimeout: cfg.Integrations.ClamAVTimeout,
			Logger:        logger.NewNop(),
		})
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("create attachment service: %w", err)
		}
		options = append(options, WithAttachmentService(attachmentService))
	}
	return NewRuntime(store, options...)
}

func NewRuntime(store *platformsql.Store, opts ...RuntimeOption) (*Runtime, error) {
	atsStore, err := NewStore(store.SqlxDB())
	if err != nil {
		return nil, err
	}
	options := &runtimeOptions{}
	for _, option := range opts {
		if option != nil {
			option(options)
		}
	}

	outboxSvc := outbox.NewService(store, eventbus.NewInMemoryBus(), logger.NewNop())
	queueService := serviceapp.NewQueueService(store.Queues(), store.QueueItems(), store.Workspaces())
	contactService := platformservices.NewContactService(store.Contacts())
	caseService := serviceapp.NewCaseService(
		store.Queues(),
		store.Cases(),
		store.Workspaces(),
		outboxSvc,
		serviceapp.WithQueueItemStore(store.QueueItems()),
		serviceapp.WithTransactionRunner(store),
	)
	extensionService := platformservices.NewExtensionServiceWithOptions(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
		options.extensionOptions...,
	)
	rulesEngine := automationservices.NewRulesEngine(
		automationservices.NewRuleService(store.Rules()),
		caseService,
		contactService,
		store.Rules(),
		outboxSvc,
	)
	rulesEngine.SetExtensionChecker(extensionService)

	service := NewService(store, atsStore, queueService, contactService, caseService, rulesEngine, extensionService, options.attachment)
	return &Runtime{
		Store:       store,
		ATSStore:    atsStore,
		Service:     service,
		Handler:     NewHandler(service),
		RulesEngine: rulesEngine,
	}, nil
}

func (r *Runtime) Register(engine *gin.Engine) {
	if r == nil || engine == nil || r.Handler == nil {
		return
	}
	RegisterRoutes(engine, r.Handler)
}

func (r *Runtime) Close() error {
	if r == nil {
		return nil
	}
	if r.RulesEngine != nil {
		r.RulesEngine.Stop()
	}
	if r.Store != nil {
		return r.Store.Close()
	}
	return nil
}
