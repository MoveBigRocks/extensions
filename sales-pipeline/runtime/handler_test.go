package salespipeline

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/extension-sdk/runtimehttp"
)

func TestExtensionModeUsesForwardedRuntimeConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("extension_config", runtimehttp.ExtensionConfig{
		"mode": "agency",
	})

	handler := NewHandler(nil)
	if got := handler.extensionMode(ctx); got != "agency" {
		t.Fatalf("expected agency mode, got %q", got)
	}
}

func TestExtensionModeDefaultsToB2B(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())

	handler := NewHandler(nil)
	if got := handler.extensionMode(ctx); got != "b2b" {
		t.Fatalf("expected b2b mode, got %q", got)
	}
}
