package main

import (
	"github.com/gin-gonic/gin"
	"log"
	"net/http"

	"github.com/movebigrocks/extension-sdk/extdb"
	"github.com/movebigrocks/extension-sdk/runtimehttp"
	"github.com/movebigrocks/extensions/operational-health/handlers"
	"github.com/movebigrocks/extensions/operational-health/runtimeui"
	"github.com/movebigrocks/extensions/operational-health/services"
	"github.com/movebigrocks/extensions/operational-health/store"
)

func main() {
	db, err := extdb.OpenFromEnv()
	if err != nil {
		log.Fatal("operational health database unavailable")
	}
	defer func() { _ = db.Close() }()
	repository := store.New(db)
	handler := &handlers.Handler{Service: services.New(repository), HealthReader: repository}
	engine := runtimehttp.DefaultEngine()
	engine.SetHTMLTemplate(runtimeui.Template())
	handlers.Register(engine, handler)
	engine.GET("/extensions/operational-health/assets/app.js", func(c *gin.Context) { c.Data(http.StatusOK, "application/javascript", runtimeui.Script) })
	if err = runtimehttp.ListenAndServeUnixSocket(engine, "demandops/operational-health"); err != nil && err != http.ErrServerClosed {
		log.Fatal("operational health runtime stopped")
	}
}
