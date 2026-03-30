package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/movebigrocks/platform/extensions/ats/runtime"
	"github.com/movebigrocks/platform/extensions/common/runtimehttp"
	"github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/config"
	"github.com/movebigrocks/platform/pkg/logger"
)

const packageKey = "demandops/ats"

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	log := logger.New().WithField("service", "ats-runtime")
	runtime, err := atsruntime.NewRuntimeFromConfig(cfg)
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
	runtime.Register(engine)

	log.Info("Starting ats runtime", "package_key", packageKey)
	if err := runtimehttp.ListenAndServeUnixSocket(engine, packageKey); err != nil && err != http.ErrServerClosed {
		log.Error("ATS runtime stopped", "error", err)
		os.Exit(1)
	}
}
