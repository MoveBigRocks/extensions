package observabilityhandlers

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/movebigrocks/extension-sdk/logger"
	observabilitydomain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
)

// ErrorEventHandler handles incoming error events from the error-events stream
// It processes raw error events and groups them into issues
type errorEventIngester interface {
	IngestEvent(context.Context, *observabilitydomain.ErrorEvent) error
}

type ErrorEventHandler struct {
	processor errorEventIngester
	logger    *logger.Logger
}

// NewErrorEventHandler creates a new error event handler
func NewErrorEventHandler(processor errorEventIngester, log *logger.Logger) *ErrorEventHandler {
	if log == nil {
		log = logger.NewNop()
	}
	return &ErrorEventHandler{
		processor: processor,
		logger:    log,
	}
}

// HandleErrorEvent processes an incoming error event
// This is the bridge between the event bus and the durable ingestion service
func (h *ErrorEventHandler) HandleErrorEvent(ctx context.Context, eventData []byte) error {
	// Parse the error event
	var event observabilitydomain.ErrorEvent
	if err := json.Unmarshal(eventData, &event); err != nil {
		return fmt.Errorf("failed to unmarshal ErrorEvent: %w", err)
	}

	h.logger.WithFields(map[string]interface{}{
		"event_id":   event.EventID,
		"project_id": event.ProjectID,
		"level":      event.Level,
	}).Info("Processing error event")

	// Process the event through durable ingestion service
	// This will group the event into an issue and publish IssueCreated/IssueUpdated events
	if err := h.processor.IngestEvent(ctx, &event); err != nil {
		h.logger.WithError(err).WithField("event_id", event.EventID).Error("Failed to process error event")
		return fmt.Errorf("failed to process error event: %w", err)
	}

	h.logger.WithField("event_id", event.EventID).Info("Successfully processed error event")
	return nil
}
