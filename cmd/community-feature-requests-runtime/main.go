package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/extension-sdk/extdb"
	"github.com/movebigrocks/extension-sdk/logger"
	"github.com/movebigrocks/extension-sdk/runtimehttp"
	communityrequests "github.com/movebigrocks/extensions/community-feature-requests/runtime"
	communityrequestsui "github.com/movebigrocks/extensions/community-feature-requests/runtimeui"
)

const packageKey = "demandops/community-feature-requests"

type communityRuntime struct {
	db      *extdb.DB
	handler *communityrequests.Handler
}

func main() {
	log := logger.New().WithField("service", "community-feature-requests-runtime")
	runtime, err := newCommunityRuntime()
	if err != nil {
		log.Error("Failed to initialize community feature requests runtime", "error", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := runtime.Close(); closeErr != nil {
			log.Warn("Failed to close community feature requests runtime", "error", closeErr)
		}
	}()

	engine := runtimehttp.DefaultEngine()
	tmpl, err := communityrequestsui.ParseTemplates()
	if err != nil {
		log.Error("Failed to parse community feature requests templates", "error", err)
		os.Exit(1)
	}
	engine.SetHTMLTemplate(tmpl)
	registerRoutes(engine, runtime.handler)

	log.Info("Starting community feature requests runtime", "package_key", packageKey)
	if err := runtimehttp.ListenAndServeUnixSocket(engine, packageKey); err != nil && err != http.ErrServerClosed {
		log.Error("Community feature requests runtime stopped", "error", err)
		os.Exit(1)
	}
}

func newCommunityRuntime() (*communityRuntime, error) {
	db, err := extdb.OpenFromEnv()
	if err != nil {
		return nil, fmt.Errorf("create database: %w", err)
	}

	requestStore, err := communityrequests.NewStore(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	return &communityRuntime{
		db:      db,
		handler: communityrequests.NewHandler(requestStore),
	}, nil
}

func (r *communityRuntime) Close() error {
	if r == nil || r.db == nil {
		return nil
	}
	return r.db.Close()
}

func registerRoutes(engine *gin.Engine, handler *communityrequests.Handler) {
	engine.GET("/community/ideas", handler.ShowBoard)
	engine.HEAD("/community/ideas", handler.ShowBoard)
	engine.POST("/community/ideas", handler.SubmitIdea)
	engine.GET("/community/ideas/:slug", handler.ShowDetail)
	engine.HEAD("/community/ideas/:slug", handler.ShowDetail)
	engine.POST("/community/ideas/:slug/vote", handler.VoteIdea)

	engine.GET("/extensions/community-feature-requests", handler.ShowAdminDashboard)
	engine.HEAD("/extensions/community-feature-requests", handler.ShowAdminDashboard)
	engine.POST("/extensions/community-feature-requests/ideas/:id", handler.UpdateIdea)

	engine.GET("/extensions/community-feature-requests/health", handler.Health)
	engine.HEAD("/extensions/community-feature-requests/health", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
}
