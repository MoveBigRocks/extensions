package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/movebigrocks/extension-sdk/runtimehost"
	"github.com/movebigrocks/extensions/operational-health/domain"
	"github.com/movebigrocks/extensions/operational-health/store"
)

type Repository interface {
	Snapshot(context.Context, string, domain.Config, domain.Source, []domain.Signal, time.Time, time.Time) error
	Evidence(context.Context, string, string, domain.EvidenceFilter) (domain.Evidence, error)
	Ingest(context.Context, string, domain.Config, domain.Source, []domain.Signal, int, bool, time.Time) (store.Receipt, error)
	Observe(context.Context, string, domain.Source, []domain.Observation, time.Time) error
	Deploy(context.Context, string, domain.Source, domain.Deployment, time.Time) error
	List(context.Context, string, domain.Filter) (domain.Page, error)
	Get(context.Context, string, string) (domain.Incident, error)
	Status(context.Context, string, domain.Config, time.Time) ([]domain.TargetStatus, error)
	Claim(context.Context, string, time.Time) (*store.Projection, error)
	Complete(context.Context, string, store.Projection, string, time.Time) error
	Fail(context.Context, string, store.Projection, int, time.Time) error
	CasesToSync(context.Context, string) ([]domain.Incident, error)
	SyncCase(context.Context, string, string, runtimehost.HostCase, time.Time) error
	SyncConfigured(context.Context, string, domain.Config, time.Time) error
	Cleanup(context.Context, string, time.Time) error
	Redrive(context.Context, string, string, string, time.Time) error
}
type Host interface {
	CreateCase(context.Context, runtimehost.CreateCaseInput) (*runtimehost.HostCase, error)
	AppendCaseNote(context.Context, string, runtimehost.AppendCaseNoteInput) (*runtimehost.HostCaseNote, error)
	GetCaseInWorkspace(context.Context, string, string) (*runtimehost.HostCase, bool, error)
}
type Service struct {
	Repo Repository
	Now  func() time.Time
}

func New(repo Repository) *Service {
	return &Service{Repo: repo, Now: func() time.Time { return time.Now().UTC() }}
}
func (s *Service) Alerts(ctx context.Context, ws string, cfg domain.Config, src domain.Source, batch domain.AlertBatch) (store.Receipt, error) {
	now := s.Now()
	signals, canary, err := domain.Normalize(src, batch, now)
	if err != nil {
		return store.Receipt{}, err
	}
	return s.Repo.Ingest(ctx, ws, cfg, src, signals, batch.Truncated, canary, now)
}
func (s *Service) Observations(ctx context.Context, ws string, src domain.Source, observations []domain.Observation) error {
	if len(observations) == 0 || len(observations) > 32 {
		return domain.ErrInvalid
	}
	now := s.Now()
	seen := map[string]bool{}
	for _, o := range observations {
		if err := o.Validate(src, now); err != nil {
			return err
		}
		if seen[o.Component] {
			return domain.ErrInvalid
		}
		seen[o.Component] = true
	}
	return s.Repo.Observe(ctx, ws, src, observations, now)
}
func (s *Service) Deployment(ctx context.Context, ws string, src domain.Source, d domain.Deployment) error {
	now := s.Now()
	if err := d.Validate(src, now); err != nil {
		return err
	}
	return s.Repo.Deploy(ctx, ws, src, d, now)
}
func (s *Service) Status(ctx context.Context, ws string, cfg domain.Config) ([]domain.TargetStatus, error) {
	return s.Repo.Status(ctx, ws, cfg, s.Now())
}
func (s *Service) List(ctx context.Context, ws string, f domain.Filter) (domain.Page, error) {
	return s.Repo.List(ctx, ws, f)
}
func (s *Service) Get(ctx context.Context, ws, id string) (domain.Incident, error) {
	return s.Repo.Get(ctx, ws, id)
}
func (s *Service) Redrive(ctx context.Context, ws, id, actor string) error {
	return s.Repo.Redrive(ctx, ws, id, actor, s.Now())
}
func (s *Service) Project(ctx context.Context, ws string, cfg domain.Config, host Host) error {
	if host == nil {
		return errors.New("host client unavailable")
	}
	if err := s.Repo.SyncConfigured(ctx, ws, cfg, s.Now()); err != nil {
		return err
	}
	for range 20 {
		if err := ctx.Err(); err != nil {
			return err
		}
		p, err := s.Repo.Claim(ctx, ws, s.Now())
		if err != nil {
			return err
		}
		if p == nil {
			break
		}
		payload, err := store.DecodeProjection(*p)
		caseID := p.CaseID
		if err == nil {
			switch p.Kind {
			case "create":
				if payload.Create == nil || payload.Create.WorkspaceID != ws {
					err = domain.ErrInvalid
					break
				}
				var result *runtimehost.HostCase
				result, err = host.CreateCase(ctx, *payload.Create)
				if err == nil {
					if result == nil || result.WorkspaceID != ws {
						err = domain.ErrForbidden
					} else {
						caseID = result.ID
					}
				}
			case "note":
				var note *runtimehost.HostCaseNote
				note, err = host.AppendCaseNote(ctx, p.CaseID, runtimehost.AppendCaseNoteInput{WorkspaceID: ws, IdempotencyKey: p.Key, Body: payload.Body})
				if err == nil && (note == nil || note.WorkspaceID != ws || note.CaseID != p.CaseID) {
					err = domain.ErrForbidden
				}
			default:
				err = domain.ErrInvalid
			}
		}
		if err != nil {
			code := runtimehost.StatusCode(err)
			if errors.Is(err, domain.ErrInvalid) || errors.Is(err, domain.ErrForbidden) {
				code = 400
			}
			if failErr := s.Repo.Fail(ctx, ws, *p, code, s.Now()); failErr != nil {
				return failErr
			}
			continue
		}
		if err = s.Repo.Complete(ctx, ws, *p, caseID, s.Now()); err != nil {
			return err
		}
	}
	// Case status is mirrored separately; it never changes a source signal or probe result.
	incidents, err := s.Repo.CasesToSync(ctx, ws)
	if err != nil {
		return err
	}
	for _, incident := range incidents {
		if incident.CaseID == "" || incident.ClosedAt != nil {
			continue
		}
		c, found, err := host.GetCaseInWorkspace(ctx, ws, incident.CaseID)
		if err != nil {
			return fmt.Errorf("read linked case: %w", err)
		}
		if found && c != nil {
			if err = s.Repo.SyncCase(ctx, ws, incident.ID, *c, s.Now()); err != nil {
				return err
			}
		}
	}
	return s.Repo.Cleanup(ctx, ws, s.Now())
}

func (s *Service) Detail(ctx context.Context, ws, id string, cfg domain.Config, f domain.EvidenceFilter) (domain.Detail, error) {
	var d domain.Detail
	i, err := s.Repo.Get(ctx, ws, id)
	if err != nil {
		return d, err
	}
	d.Incident = i
	d.Evidence, err = s.Repo.Evidence(ctx, ws, id, f)
	if err != nil {
		return d, err
	}
	targets, err := s.Status(ctx, ws, cfg)
	if err != nil {
		return d, err
	}
	for _, t := range targets {
		if t.SourceID == i.SourceID && t.Component == i.Component {
			d.Target = &t
			break
		}
	}
	return d, nil
}

func (s *Service) Snapshot(ctx context.Context, ws string, cfg domain.Config, src domain.Source, snapshot domain.Snapshot) error {
	now := s.Now()
	signals, err := snapshot.Normalize(src, now)
	if err != nil {
		return err
	}
	return s.Repo.Snapshot(ctx, ws, cfg, src, signals, snapshot.ObservedAt, now)
}
