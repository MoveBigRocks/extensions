package atsruntime

import (
	"fmt"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/extension-sdk/extdb"
)

// Runtime is the ATS extension runtime. It owns the ext_demandops_ats schema
// through its own database connection and reaches all core data (contacts,
// cases, queues, attachments, rules, artifacts) through the platform host API,
// never by importing platform internals.
type Runtime struct {
	Store   *Store
	Service *Service
	Handler *Handler
}

// NewRuntimeFromEnv opens the extension database from the environment and wires
// the ATS runtime over the platform host API.
func NewRuntimeFromEnv() (*Runtime, error) {
	db, err := extdb.OpenFromEnv()
	if err != nil {
		return nil, fmt.Errorf("open ats database: %w", err)
	}
	rt, err := NewRuntime(db, hostFromContext)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return rt, nil
}

// NewRuntime wires the ATS runtime over the given database and host provider.
// Production passes hostFromContext, which resolves the per-request host client
// the middleware places on the context; tests pass a fake provider.
func NewRuntime(db *extdb.DB, newHost hostProvider) (*Runtime, error) {
	store, err := NewStore(db)
	if err != nil {
		return nil, err
	}
	service := NewService(store, newHost)
	return &Runtime{
		Store:   store,
		Service: service,
		Handler: NewHandler(service),
	}, nil
}

func (r *Runtime) Register(engine *gin.Engine) {
	if r == nil || engine == nil || r.Handler == nil {
		return
	}
	RegisterRoutes(engine, r.Handler)
}

func (r *Runtime) Close() error {
	if r == nil || r.Store == nil {
		return nil
	}
	return r.Store.Close()
}
