package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/movebigrocks/extension-sdk/runtimehost"
	"github.com/movebigrocks/extension-sdk/runtimehttp"
	"github.com/movebigrocks/extension-sdk/runtimeproto"
	"github.com/movebigrocks/extensions/operational-health/domain"
	"github.com/movebigrocks/extensions/operational-health/services"
)

type HealthReader interface{ Health(context.Context) error }
type Handler struct {
	Service      *services.Service
	HealthReader HealthReader
}

func (h *Handler) config(c *gin.Context) (string, domain.Config, bool) {
	ws := c.GetString("workspace_id")
	var cfg domain.Config
	data, err := json.Marshal(runtimehttp.ExtensionConfigMap(c))
	if err != nil {
		respond(c, err)
		return "", cfg, false
	}
	if err = json.Unmarshal(data, &cfg); err != nil || cfg.Validate() != nil || ws == "" {
		respond(c, errors.New("operational configuration unavailable"))
		return "", cfg, false
	}
	return ws, cfg, true
}
func (h *Handler) source(c *gin.Context) (string, domain.Config, domain.Source, bool) {
	ws, cfg, ok := h.config(c)
	if !ok {
		return "", cfg, domain.Source{}, false
	}
	scheme, token, _ := strings.Cut(c.GetHeader("Authorization"), " ")
	if !strings.EqualFold(scheme, "Bearer") {
		respond(c, domain.ErrForbidden)
		return "", cfg, domain.Source{}, false
	}
	src, err := cfg.Authenticate(c.Param("sourceID"), strings.TrimSpace(token))
	if err != nil {
		respond(c, err)
		return "", cfg, src, false
	}
	return ws, cfg, src, true
}
func decode(c *gin.Context, dest any) bool {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	decoder := json.NewDecoder(c.Request.Body)
	if err := decoder.Decode(dest); err != nil {
		respond(c, domain.ErrInvalid)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		respond(c, domain.ErrInvalid)
		return false
	}
	return true
}
func respond(c *gin.Context, err error) {
	code, status := "unavailable", http.StatusServiceUnavailable
	switch {
	case errors.Is(err, domain.ErrInvalid):
		code, status = "invalid_arguments", 400
	case errors.Is(err, domain.ErrForbidden):
		code, status = "forbidden", 403
	case errors.Is(err, domain.ErrNotFound):
		code, status = "not_found", 404
	case errors.Is(err, domain.ErrConflict):
		code, status = "conflict", 409
	}
	c.AbortWithStatusJSON(status, gin.H{"error": gin.H{"code": code, "message": http.StatusText(status)}})
}
func envelope(ws string) gin.H { return gin.H{"contract": domain.ContractVersion, "workspaceId": ws} }
func (h *Handler) Alerts(c *gin.Context) {
	ws, cfg, src, ok := h.source(c)
	if !ok {
		return
	}
	var batch domain.AlertBatch
	if !decode(c, &batch) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	result, err := h.Service.Alerts(ctx, ws, cfg, src, batch)
	if err != nil {
		respond(c, err)
		return
	}
	out := envelope(ws)
	out["receipt"] = result
	c.JSON(http.StatusAccepted, out)
}
func (h *Handler) Observations(c *gin.Context) {
	ws, _, src, ok := h.source(c)
	if !ok {
		return
	}
	var body struct {
		Observations []domain.Observation `json:"observations"`
	}
	if !decode(c, &body) {
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
	defer cancel()
	if err := h.Service.Observations(ctx, ws, src, body.Observations); err != nil {
		respond(c, err)
		return
	}
	c.JSON(http.StatusAccepted, envelope(ws))
}
func (h *Handler) Deployment(c *gin.Context) {
	ws, _, src, ok := h.source(c)
	if !ok {
		return
	}
	var body domain.Deployment
	if !decode(c, &body) {
		return
	}
	if err := h.Service.Deployment(c.Request.Context(), ws, src, body); err != nil {
		respond(c, err)
		return
	}
	c.JSON(http.StatusAccepted, envelope(ws))
}
func (h *Handler) Status(c *gin.Context) {
	ws, cfg, ok := h.config(c)
	if !ok {
		return
	}
	result, err := h.Service.Status(c.Request.Context(), ws, cfg)
	if err != nil {
		respond(c, err)
		return
	}
	out := envelope(ws)
	out["targets"] = result
	out["checkedAt"] = h.Service.Now()
	out["configured"] = len(result) > 0
	c.JSON(http.StatusOK, out)
}
func (h *Handler) List(c *gin.Context) {
	ws, _, ok := h.config(c)
	if !ok {
		return
	}
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil {
			respond(c, domain.ErrInvalid)
			return
		}
	}
	result, err := h.Service.List(c.Request.Context(), ws, domain.Filter{Environment: c.Query("environment"), Component: c.Query("component"), State: c.Query("state"), Severity: c.Query("severity"), Owner: c.Query("owner"), After: c.Query("after"), Limit: limit})
	if err != nil {
		respond(c, err)
		return
	}
	out := envelope(ws)
	out["incidents"] = result
	c.JSON(http.StatusOK, out)
}
func (h *Handler) Get(c *gin.Context) {
	ws, cfg, ok := h.config(c)
	if !ok {
		return
	}
	limit := 20
	if raw := c.Query("limit"); raw != "" {
		var err error
		limit, err = strconv.Atoi(raw)
		if err != nil {
			respond(c, domain.ErrInvalid)
			return
		}
	}
	d, err := h.Service.Detail(c.Request.Context(), ws, c.Param("id"), cfg, domain.EvidenceFilter{Limit: limit, SignalsAfter: c.Query("signalsAfter"), ProjectionsAfter: c.Query("projectionsAfter")})
	if err != nil {
		respond(c, err)
		return
	}
	out := envelope(ws)
	out["incident"] = d.Incident
	out["evidence"] = d.Evidence
	out["target"] = d.Target
	c.JSON(http.StatusOK, out)
}
func (h *Handler) Redrive(c *gin.Context) {
	ws, _, ok := h.config(c)
	if !ok {
		return
	}
	if err := h.Service.Redrive(c.Request.Context(), ws, c.Param("id"), c.GetString("user_id")); err != nil {
		respond(c, err)
		return
	}
	c.JSON(http.StatusAccepted, envelope(ws))
}
func (h *Handler) Project(c *gin.Context) {
	if c.GetHeader(runtimeproto.HeaderInternalRequest) != "true" {
		c.AbortWithStatus(404)
		return
	}
	ws, cfg, ok := h.config(c)
	if !ok {
		return
	}
	host, err := runtimehost.NewClientFromRequest(c.Request)
	if err != nil {
		respond(c, err)
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 25*time.Second)
	defer cancel()
	if err = h.Service.Project(ctx, ws, cfg, host); err != nil {
		respond(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
func (h *Handler) Health(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	if err := h.HealthReader.Health(ctx); err != nil {
		respond(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "healthy", "contract": domain.ContractVersion})
}
func (h *Handler) Dashboard(c *gin.Context) {
	data := runtimehttp.BuildBasePageData(c, "operational-health", "Incidents", "Current health and incident response")
	c.HTML(http.StatusOK, "dashboard.html", data)
}
func Register(engine *gin.Engine, h *Handler) {
	engine.Use(func(c *gin.Context) {
		timeout := 5 * time.Second
		if c.Request.URL.Path == runtimeproto.InternalJobPath("operational-health.project") {
			timeout = 25 * time.Second
		}
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	source := "/api/operational-health/sources/:sourceID"
	engine.POST(source+"/alerts", h.Alerts)
	engine.POST(source+"/observations", h.Observations)
	engine.POST(source+"/deployments", h.Deployment)
	engine.POST(source+"/snapshot", h.Snapshot)
	for _, prefix := range []string{"/extensions/operational-health/api", "/extensions/operational-health/api/agent"} {
		engine.GET(prefix+"/status", h.Status)
		engine.GET(prefix+"/incidents", h.List)
		engine.GET(prefix+"/incidents/:id", h.Get)
	}
	engine.POST("/extensions/operational-health/api/projections/:id/retry", h.Redrive)
	engine.POST(runtimeproto.InternalJobPath("operational-health.project"), h.Project)
	engine.GET("/extensions/operational-health/health", h.Health)
	engine.GET("/extensions/operational-health", h.Dashboard)
}

func (h *Handler) Snapshot(c *gin.Context) {
	ws, cfg, src, ok := h.source(c)
	if !ok {
		return
	}
	var snapshot domain.Snapshot
	if !decode(c, &snapshot) {
		return
	}
	if err := h.Service.Snapshot(c.Request.Context(), ws, cfg, src, snapshot); err != nil {
		respond(c, err)
		return
	}
	c.JSON(http.StatusAccepted, envelope(ws))
}
