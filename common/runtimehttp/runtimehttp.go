package runtimehttp

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	sdkruntimehttp "github.com/movebigrocks/extension-sdk/runtimehttp"
)

type SessionContext = sdkruntimehttp.SessionContext

type Session = sdkruntimehttp.Session

type ExtensionConfig = sdkruntimehttp.ExtensionConfig

func DefaultEngine() *gin.Engine {
	return sdkruntimehttp.DefaultEngine()
}

func ListenAndServeUnixSocket(handler http.Handler, packageKey string) error {
	return sdkruntimehttp.ListenAndServeUnixSocket(handler, packageKey)
}

func RegisterInternalRoutes(
	engine *gin.Engine,
	eventConsumers map[string]func(context.Context, []byte) error,
	jobs map[string]func(context.Context) error,
) {
	sdkruntimehttp.RegisterInternalRoutes(engine, eventConsumers, jobs)
}

func BuildBasePageData(c *gin.Context, activePage, title, subtitle string) gin.H {
	return sdkruntimehttp.BuildBasePageData(c, activePage, title, subtitle)
}

func ExtensionID(c *gin.Context) string {
	return sdkruntimehttp.ExtensionID(c)
}

func ExtensionSlug(c *gin.Context) string {
	return sdkruntimehttp.ExtensionSlug(c)
}

func ExtensionPackageKey(c *gin.Context) string {
	return sdkruntimehttp.ExtensionPackageKey(c)
}

func ExtensionConfigMap(c *gin.Context) ExtensionConfig {
	return sdkruntimehttp.ExtensionConfigMap(c)
}

func ExtensionConfigString(c *gin.Context, key string) (string, bool) {
	return sdkruntimehttp.ExtensionConfigString(c, key)
}

func ForwardedContextMiddleware() gin.HandlerFunc {
	return sdkruntimehttp.ForwardedContextMiddleware()
}
