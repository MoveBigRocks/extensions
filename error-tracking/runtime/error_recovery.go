package sql

import (
	"context"
	"fmt"

	domain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
	models "github.com/movebigrocks/extensions/error-tracking/sql-models"
)

// ClaimPendingErrorEvents leases a bounded batch using the last processing
// attempt. A crash after claiming delays retry by one minute; it never deletes
// the raw event. Ordering by last attempt prevents a failed event starving the
// backlog. SKIP LOCKED permits overlapping host jobs without duplicate claims.
func (s *ErrorMonitoringStore) ClaimPendingErrorEvents(ctx context.Context, limit int) ([]*domain.ErrorEvent, error) {
	if limit < 1 || limit > 100 {
		return nil, fmt.Errorf("recovery batch must contain 1 to 100 events")
	}
	var result []*domain.ErrorEvent
	err := s.db.Transaction(ctx, func(tx context.Context) error {
		var rows []models.ErrorEvent
		err := s.selectContext(tx, &rows, `WITH pending AS (
   SELECT e.id FROM ${SCHEMA_NAME}.error_events e
   JOIN ${SCHEMA_NAME}.projects p ON p.id = e.project_id AND p.deleted_at IS NULL
   WHERE e.issue_id IS NULL AND COALESCE(e.processed_at, e.received) <= NOW() - INTERVAL '1 minute'
   ORDER BY COALESCE(e.processed_at, e.received), e.id
   LIMIT ? FOR UPDATE OF e SKIP LOCKED
  ) UPDATE ${SCHEMA_NAME}.error_events e SET processed_at = NOW()
    FROM pending WHERE e.id = pending.id RETURNING e.*`, limit)
		if err != nil {
			return err
		}
		for _, row := range rows {
			event, err := s.mapEventToDomain(&row)
			if err != nil {
				return err
			}
			result = append(result, event)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
