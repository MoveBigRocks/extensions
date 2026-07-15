package main

import (
	"net/http"
	"os"

	"github.com/movebigrocks/extension-sdk/logger"
	"github.com/movebigrocks/extension-sdk/runtimehttp"
	atsruntime "github.com/movebigrocks/extensions/ats/runtime"
)

const packageKey = "demandops/ats"

func main() {
	log := logger.New().WithField("service", "ats-runtime")
	runtime, err := atsruntime.NewRuntimeFromEnv()
	if err != nil {
		log.Error("Failed to initialize ats runtime", "error", err)
		os.Exit(1)
	}
	defer func() {
		if closeErr := runtime.Close(); closeErr != nil {
			log.Warn("Failed to close ats runtime", "error", closeErr)
		}
	}()

	engine := runtimehttp.DefaultEngine()
	// ATS reaches core data through the platform host API. Build a host client
	// from the token and base URL the platform forwards on each proxied request
	// and put it on the request context so handlers can call back into core.
	engine.Use(atsruntime.HostClientMiddleware())
	runtime.Register(engine)

	log.Info("Starting ats runtime", "package_key", packageKey)
	if err := runtimehttp.ListenAndServeUnixSocket(engine, packageKey); err != nil && err != http.ErrServerClosed {
		log.Error("ATS runtime stopped", "error", err)
		os.Exit(1)
	}
}
