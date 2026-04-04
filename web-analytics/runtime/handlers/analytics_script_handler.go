package analyticshandlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// AnalyticsScriptHandler serves the tracking script at GET /js/analytics.js.
type AnalyticsScriptHandler struct {
	scriptContent []byte
}

func NewAnalyticsScriptHandlerWithContent(content []byte) *AnalyticsScriptHandler {
	return &AnalyticsScriptHandler{scriptContent: append([]byte(nil), content...)}
}

// ServeScript handles GET /js/analytics.js with aggressive caching.
func (h *AnalyticsScriptHandler) ServeScript(c *gin.Context) {
	if len(h.scriptContent) == 0 {
		c.Status(http.StatusNotFound)
		return
	}

	c.Header("Content-Type", "application/javascript; charset=utf-8")
	c.Header("Cache-Control", "public, max-age=86400")
	c.Data(http.StatusOK, "application/javascript; charset=utf-8", h.scriptContent)
}
