package runtimehttp

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	sdkruntimehttp "github.com/movebigrocks/extension-sdk/runtimehttp"
)

type SessionContext = sdkruntimehttp.SessionContext

type Session = sdkruntimehttp.Session

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

func ForwardedContextMiddleware() gin.HandlerFunc {
	return sdkruntimehttp.ForwardedContextMiddleware()
}
