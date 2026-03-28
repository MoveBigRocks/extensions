package atsruntime

import (
	"fmt"

	"github.com/gin-gonic/gin"

	automationservices "github.com/movebigrocks/platform/internal/automation/services"
	"github.com/movebigrocks/platform/internal/infrastructure/config"
	"github.com/movebigrocks/platform/internal/infrastructure/outbox"
	platformsql "github.com/movebigrocks/platform/internal/infrastructure/stores/sql"
	platformservices "github.com/movebigrocks/platform/internal/platform/services"
	serviceapp "github.com/movebigrocks/platform/internal/service/services"
	"github.com/movebigrocks/platform/pkg/eventbus"
	"github.com/movebigrocks/platform/pkg/logger"
)

type Runtime struct {
	Store       *platformsql.Store
	ATSStore    *Store
	Service     *Service
	Handler     *Handler
	RulesEngine *automationservices.RulesEngine
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
	return NewRuntime(store)
}

func NewRuntime(store *platformsql.Store) (*Runtime, error) {
	atsStore, err := NewStore(store.SqlxDB())
	if err != nil {
		return nil, err
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
	extensionService := platformservices.NewExtensionService(
		store.Extensions(),
		store.Workspaces(),
		store.Queues(),
		store.Forms(),
		store.Rules(),
		store,
	)
	rulesEngine := automationservices.NewRulesEngine(
		automationservices.NewRuleService(store.Rules()),
		caseService,
		contactService,
		store.Rules(),
		outboxSvc,
	)
	rulesEngine.SetExtensionChecker(extensionService)

	service := NewService(store, atsStore, queueService, contactService, caseService, rulesEngine)
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
