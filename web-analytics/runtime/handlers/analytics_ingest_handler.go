package analyticshandlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/movebigrocks/extension-sdk/extensionhost/infrastructure/metrics"
	"github.com/movebigrocks/extension-sdk/logger"
	analyticsservices "github.com/movebigrocks/extensions/web-analytics/runtime/services"
)

// analyticsRateLimiter tracks request counts for analytics ingest.
type analyticsRateLimiter struct {
	mu       sync.RWMutex
	counters map[string]*analyticsRateCounter
}

type analyticsRateCounter struct {
	count     int
	resetTime time.Time
}

var (
	ipRateLimiter       = &analyticsRateLimiter{counters: make(map[string]*analyticsRateCounter)}
	propertyRateLimiter = &analyticsRateLimiter{counters: make(map[string]*analyticsRateCounter)}
)

const (
	ipRateLimit                 = 100   // events per minute per IP
	propertyRateLimit           = 10000 // events per minute per property
	rateLimitWindow             = time.Minute
	analyticsIngestMaxBodyBytes = 64 * 1024
)

func (rl *analyticsRateLimiter) allow(key string, limit int) bool {
	now := time.Now()

	rl.mu.Lock()
	defer rl.mu.Unlock()

	counter, exists := rl.counters[key]
	if !exists || now.After(counter.resetTime) {
		rl.counters[key] = &analyticsRateCounter{count: 1, resetTime: now.Add(rateLimitWindow)}
		return true
	}

	counter.count++
	return counter.count <= limit
}

// StartAnalyticsRateLimiterCleanup starts a goroutine to clean up expired rate limit entries.
func StartAnalyticsRateLimiterCleanup(stop <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				CleanupAnalyticsRateLimiters()
			case <-stop:
				return
			}
		}
	}()
}

func CleanupAnalyticsRateLimiters() {
	cleanupRateLimiter(ipRateLimiter)
	cleanupRateLimiter(propertyRateLimiter)
}

func cleanupRateLimiter(rl *analyticsRateLimiter) {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for key, counter := range rl.counters {
		if now.After(counter.resetTime) {
			delete(rl.counters, key)
		}
	}
}

// AnalyticsIngestHandler handles the public POST /api/analytics/event endpoint.
type AnalyticsIngestHandler struct {
	ingestService *analyticsservices.IngestService
	logger        *logger.Logger
}

// NewAnalyticsIngestHandler creates a new analytics ingest handler.
func NewAnalyticsIngestHandler(
	ingestService *analyticsservices.IngestService,
	log *logger.Logger,
) *AnalyticsIngestHandler {
	if log == nil {
		log = logger.NewNop()
	}
	return &AnalyticsIngestHandler{
		ingestService: ingestService,
		logger:        log,
	}
}

type ingestRevenuePayload struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

type ingestLocationPayload struct {
	Country string `json:"country"`
	Region  string `json:"region"`
	City    string `json:"city"`
}

// ingestPayload represents the JSON body from the tracking script.
type ingestPayload struct {
	Name     string                 `json:"n"`
	URL      string                 `json:"u"`
	Domain   string                 `json:"d"`
	Referrer string                 `json:"r"`
	Honeypot string                 `json:"p"` // Undocumented honeypot field
	Props    map[string]interface{} `json:"h"`
	Revenue  *ingestRevenuePayload  `json:"v"`
	Location *ingestLocationPayload `json:"l"`
}

// HandleEvent handles POST /api/analytics/event.
// Always returns 202 regardless of outcome to prevent domain enumeration.
func (h *AnalyticsIngestHandler) HandleEvent(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, analyticsIngestMaxBodyBytes)

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			c.Status(http.StatusRequestEntityTooLarge)
			return
		}
		c.Status(http.StatusAccepted)
		return
	}

	var payload ingestPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		c.Status(http.StatusAccepted)
		return
	}

	// Validate required fields
	if payload.Name == "" || payload.URL == "" || payload.Domain == "" {
		c.Status(http.StatusAccepted)
		return
	}

	clientIP := ResolveClientIP(c)
	ua := c.GetHeader("User-Agent")

	// Honeypot check
	if payload.Honeypot != "" {
		h.logger.Info("analytics.honeypot", "ip", clientIP, "domain", payload.Domain)
		metrics.AnalyticsEventsRejectedTotal.WithLabelValues("honeypot").Inc()
		c.Status(http.StatusAccepted)
		return
	}

	// Server-side bot check
	if IsBotUA(ua) {
		h.logger.Info("analytics.bot", "ip", clientIP, "ua", ua, "domain", payload.Domain)
		metrics.AnalyticsEventsRejectedTotal.WithLabelValues("bot").Inc()
		c.Status(http.StatusAccepted)
		return
	}

	// Rate limit by IP
	if !ipRateLimiter.allow("ip:"+clientIP, ipRateLimit) {
		h.logger.Info("analytics.ratelimit", "ip", clientIP, "window", "60s")
		metrics.AnalyticsEventsRejectedTotal.WithLabelValues("ratelimit").Inc()
		c.Status(http.StatusAccepted)
		return
	}

	// Referrer spam check
	if payload.Referrer != "" && IsReferrerSpam(payload.Referrer) {
		h.logger.Info("analytics.spam", "ip", clientIP, "referrer", payload.Referrer, "domain", payload.Domain)
		metrics.AnalyticsEventsRejectedTotal.WithLabelValues("spam").Inc()
		c.Status(http.StatusAccepted)
		return
	}

	// Truncate fields to max lengths
	payload.Name = truncate(payload.Name, 120)
	payload.URL = truncate(payload.URL, 2000)
	payload.Domain = truncate(payload.Domain, 253)
	payload.Referrer = truncate(payload.Referrer, 2000)

	// Strip control characters from event name
	payload.Name = stripControlChars(payload.Name)

	customProps := normalizeAnalyticsProps(payload.Props)
	revenueCurrency, revenueAmountCents := normalizeAnalyticsRevenue(payload.Revenue)
	countryCode, region, city := normalizeAnalyticsLocation(payload.Location)

	// Rate limit by property domain
	if !propertyRateLimiter.allow("prop:"+payload.Domain, propertyRateLimit) {
		h.logger.Info("analytics.ratelimit", "ip", clientIP, "domain", payload.Domain, "window", "60s", "scope", "property")
		metrics.AnalyticsEventsRejectedTotal.WithLabelValues("ratelimit").Inc()
		c.Status(http.StatusAccepted)
		return
	}

	// Process the event
	req := &analyticsservices.IngestRequest{
		EventName:          payload.Name,
		URL:                payload.URL,
		Domain:             payload.Domain,
		Referrer:           payload.Referrer,
		UserAgent:          ua,
		RemoteIP:           clientIP,
		AcceptLang:         c.GetHeader("Accept-Language"),
		CustomProperties:   customProps,
		RevenueCurrency:    revenueCurrency,
		RevenueAmountCents: revenueAmountCents,
		CountryCode:        countryCode,
		Region:             region,
		City:               city,
	}

	if err := h.ingestService.Ingest(c.Request.Context(), req); err != nil {
		h.logger.Warn("Failed to ingest analytics event", "error", err)
	} else {
		metrics.AnalyticsEventsIngestedTotal.WithLabelValues(payload.Domain).Inc()
	}

	c.Status(http.StatusAccepted)
}

func truncate(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	// Walk back from the cut point to find a valid UTF-8 boundary
	for maxBytes > 0 && !utf8.RuneStart(s[maxBytes]) {
		maxBytes--
	}
	return s[:maxBytes]
}

func stripControlChars(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 0x20 {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normalizeAnalyticsProps(raw map[string]interface{}) map[string]string {
	if len(raw) == 0 {
		return nil
	}
	props := make(map[string]string, len(raw))
	count := 0
	for key, value := range raw {
		if count >= 32 {
			break
		}
		key = truncate(stripControlChars(strings.TrimSpace(key)), 64)
		if key == "" {
			continue
		}

		rendered := analyticsPropValueString(value)
		rendered = truncate(stripControlChars(strings.TrimSpace(rendered)), 256)
		if rendered == "" {
			continue
		}
		props[key] = rendered
		count++
	}
	if len(props) == 0 {
		return nil
	}
	return props
}

func analyticsPropValueString(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(typed), 'f', -1, 32)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

func normalizeAnalyticsRevenue(payload *ingestRevenuePayload) (string, *int64) {
	if payload == nil {
		return "", nil
	}
	if math.IsNaN(payload.Amount) || math.IsInf(payload.Amount, 0) || payload.Amount < 0 {
		return "", nil
	}
	currency := strings.ToUpper(truncate(stripControlChars(strings.TrimSpace(payload.Currency)), 16))
	if currency == "" {
		return "", nil
	}
	amountCents := int64(math.Round(payload.Amount * 100))
	return currency, &amountCents
}

func normalizeAnalyticsLocation(payload *ingestLocationPayload) (string, string, string) {
	if payload == nil {
		return "", "", ""
	}
	country := strings.ToUpper(truncate(stripControlChars(strings.TrimSpace(payload.Country)), 8))
	region := truncate(stripControlChars(strings.TrimSpace(payload.Region)), 120)
	city := truncate(stripControlChars(strings.TrimSpace(payload.City)), 120)
	return country, region, city
}
