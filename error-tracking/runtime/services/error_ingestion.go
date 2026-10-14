package observabilityservices

import (
	"context"
	"errors"
	"fmt"
	"time"

	domain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
)

type ingestionStore interface {
	GetProjectByKey(context.Context, string) (*domain.Project, error)
	AcceptSentryEvent(context.Context, *domain.ErrorEvent) (bool, error)
	ClaimPendingErrorEvents(context.Context, int) ([]*domain.ErrorEvent, error)
}

type ingestionScoper interface {
	WithAdminContext(context.Context, func(context.Context) error) error
}

type eventGrouping interface {
	GroupEvent(context.Context, *domain.ErrorEvent) (*domain.Issue, bool, error)
}

// ErrorIngestion commits raw acceptance before publishing grouping work to the
// host's durable outbox. Every public method owns its scope; callers must not
// wrap it in a request-wide transaction that would postpone these commits.
type ErrorIngestion struct {
	store    ingestionStore
	grouping eventGrouping
	scoper   ingestionScoper
}

func NewErrorIngestion(store ingestionStore, grouping eventGrouping, scoper ingestionScoper) *ErrorIngestion {
	return &ErrorIngestion{store: store, grouping: grouping, scoper: scoper}
}

func (s *ErrorIngestion) GetProjectByKey(ctx context.Context, key string) (*domain.Project, error) {
	var project *domain.Project
	err := s.scoper.WithAdminContext(ctx, func(tx context.Context) error {
		var err error
		project, err = s.store.GetProjectByKey(tx, key)
		return err
	})
	return project, err
}

func (s *ErrorIngestion) AcceptSentryEvent(ctx context.Context, event *domain.ErrorEvent) (bool, error) {
	var created bool
	err := s.scoper.WithAdminContext(ctx, func(tx context.Context) error {
		var err error
		created, err = s.store.AcceptSentryEvent(tx, event)
		return err
	})
	return created, err
}

// ProcessEvent returns only after host publication has acknowledged durable
// work. Projection may happen later. A speculative issue ID never becomes an
// HTTP receipt; only a link read from the persisted source event is exposed.
func (s *ErrorIngestion) ProcessEvent(ctx context.Context, event *domain.ErrorEvent) error {
	if event.IssueID != "" {
		return nil
	}
	pending := *event
	return s.scoper.WithAdminContext(ctx, func(tx context.Context) error {
		_, _, err := s.grouping.GroupEvent(tx, &pending)
		return err
	})
}

func (s *ErrorIngestion) IngestEvent(ctx context.Context, event *domain.ErrorEvent) error {
	if _, err := s.AcceptSentryEvent(ctx, event); err != nil {
		return err
	}
	return s.ProcessEvent(ctx, event)
}

// RecoverPendingEvents is invoked by the host scheduler with a fresh scoped
// host credential after every restart. Claims commit before any network call;
// a failed or interrupted batch becomes eligible again without losing work.
func (s *ErrorIngestion) RecoverPendingEvents(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var events []*domain.ErrorEvent
	err := s.scoper.WithAdminContext(ctx, func(tx context.Context) error {
		var err error
		events, err = s.store.ClaimPendingErrorEvents(tx, 25)
		return err
	})
	if err != nil {
		return err
	}
	var failures []error
	for _, event := range events {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		if err := s.ProcessEvent(ctx, event); err != nil {
			failures = append(failures, fmt.Errorf("recover event %s: %w", event.ID, err))
		}
	}
	return errors.Join(failures...)
}
