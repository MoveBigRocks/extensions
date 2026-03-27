package salespipeline

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	platformsql "github.com/movebigrocks/platform/internal/infrastructure/stores/sql"
)

type Store struct {
	db *platformsql.SqlxDB
}

func NewStore(db *platformsql.SqlxDB) (*Store, error) {
	store := &Store{db: db}
	if err := store.ensureSchemaAvailable(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) ensureSchemaAvailable(ctx context.Context) error {
	var regclass sql.NullString
	if err := s.db.Get(ctx).GetContext(ctx, &regclass, s.query(`SELECT to_regclass(?)`), SchemaName+".deals"); err != nil {
		return fmt.Errorf("check sales pipeline schema availability: %w", err)
	}
	if !regclass.Valid || regclass.String == "" {
		return fmt.Errorf("sales pipeline schema %s is not available", SchemaName)
	}
	return nil
}

func (s *Store) Board(ctx context.Context, workspaceID, mode string) (*Board, error) {
	stages, err := s.ensureDefaultStages(ctx, workspaceID, mode)
	if err != nil {
		return nil, err
	}
	deals, err := s.listDeals(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	columns := make([]StageColumn, 0, len(stages))
	byStage := make(map[string][]Deal, len(stages))
	for _, deal := range deals {
		byStage[deal.StageID] = append(byStage[deal.StageID], deal)
	}

	var totalCents int64
	totalDeals := 0
	for _, stage := range stages {
		stageDeals := byStage[stage.ID]
		var stageTotal int64
		for _, deal := range stageDeals {
			stageTotal += deal.ValueCents
		}
		totalCents += stageTotal
		totalDeals += len(stageDeals)
		columns = append(columns, StageColumn{
			Stage:      stage,
			Deals:      stageDeals,
			TotalCents: stageTotal,
			TotalCount: len(stageDeals),
		})
	}

	return &Board{
		Mode: mode,
		Summary: BoardSummary{
			TotalDeals: totalDeals,
			TotalCents: totalCents,
		},
		Stages: columns,
	}, nil
}

func (s *Store) CreateDeal(ctx context.Context, input CreateDealInput, mode string) (*Deal, error) {
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if strings.TrimSpace(input.Title) == "" {
		return nil, fmt.Errorf("deal title is required")
	}
	if input.WinProbability < 0 || input.WinProbability > 100 {
		return nil, fmt.Errorf("win probability must be between 0 and 100")
	}

	stages, err := s.ensureDefaultStages(ctx, workspaceID, mode)
	if err != nil {
		return nil, err
	}
	if len(stages) == 0 {
		return nil, fmt.Errorf("no stages configured")
	}

	stageID := stages[0].ID
	now := time.Now().UTC()
	currency := strings.ToUpper(strings.TrimSpace(input.Currency))
	if currency == "" {
		currency = "USD"
	}

	deal := &Deal{
		ID:               uuid.NewString(),
		Workspace:        workspaceID,
		StageID:          stageID,
		Title:            strings.TrimSpace(input.Title),
		OrganizationName: strings.TrimSpace(input.OrganizationName),
		ContactName:      strings.TrimSpace(input.ContactName),
		ContactEmail:     strings.TrimSpace(input.ContactEmail),
		LinkedCaseID:     strings.TrimSpace(input.LinkedCaseID),
		ValueCents:       input.ValueCents,
		Currency:         currency,
		CloseDate:        input.CloseDate,
		WinProbability:   input.WinProbability,
		Notes:            strings.TrimSpace(input.Notes),
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if err := s.db.Get(ctx).GetContext(ctx, deal, s.query(`
		INSERT INTO ${SCHEMA_NAME}.deals (
			id, workspace_id, stage_id, title, organization_name, contact_name, contact_email,
			linked_case_id, value_cents, currency, close_date, win_probability, notes, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, workspace_id, stage_id, title, organization_name, contact_name, contact_email,
			linked_case_id, value_cents, currency, close_date, win_probability, notes, created_at, updated_at
	`),
		deal.ID,
		deal.Workspace,
		deal.StageID,
		deal.Title,
		deal.OrganizationName,
		deal.ContactName,
		deal.ContactEmail,
		deal.LinkedCaseID,
		deal.ValueCents,
		deal.Currency,
		deal.CloseDate,
		deal.WinProbability,
		deal.Notes,
		deal.CreatedAt,
		deal.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("create deal: %w", err)
	}
	return deal, nil
}

func (s *Store) MoveDeal(ctx context.Context, workspaceID, dealID, stageID string) (*Deal, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	dealID = strings.TrimSpace(dealID)
	stageID = strings.TrimSpace(stageID)
	if workspaceID == "" || dealID == "" || stageID == "" {
		return nil, fmt.Errorf("workspace, deal, and stage are required")
	}

	var stageCount int
	if err := s.db.Get(ctx).GetContext(ctx, &stageCount, s.query(`
		SELECT COUNT(*) FROM ${SCHEMA_NAME}.stages WHERE workspace_id = ? AND id = ?
	`), workspaceID, stageID); err != nil {
		return nil, fmt.Errorf("check stage: %w", err)
	}
	if stageCount == 0 {
		return nil, fmt.Errorf("stage not found")
	}

	deal := &Deal{}
	if err := s.db.Get(ctx).GetContext(ctx, deal, s.query(`
		UPDATE ${SCHEMA_NAME}.deals
		SET stage_id = ?, updated_at = NOW()
		WHERE workspace_id = ? AND id = ?
		RETURNING id, workspace_id, stage_id, title, organization_name, contact_name, contact_email,
			linked_case_id, value_cents, currency, close_date, win_probability, notes, created_at, updated_at
	`), stageID, workspaceID, dealID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("deal not found")
		}
		return nil, fmt.Errorf("move deal: %w", err)
	}
	return deal, nil
}

func (s *Store) ensureDefaultStages(ctx context.Context, workspaceID, mode string) ([]Stage, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	var count int
	if err := s.db.Get(ctx).GetContext(ctx, &count, s.query(`
		SELECT COUNT(*) FROM ${SCHEMA_NAME}.stages WHERE workspace_id = ?
	`), workspaceID); err != nil {
		return nil, fmt.Errorf("count stages: %w", err)
	}
	if count == 0 {
		if err := s.db.Transaction(ctx, func(txCtx context.Context) error {
			var stageCount int
			if err := s.db.Get(txCtx).GetContext(txCtx, &stageCount, s.query(`
				SELECT COUNT(*) FROM ${SCHEMA_NAME}.stages WHERE workspace_id = ?
			`), workspaceID); err != nil {
				return err
			}
			if stageCount > 0 {
				return nil
			}
			now := time.Now().UTC()
			for idx, seed := range defaultStageSeeds(mode) {
				if _, err := s.db.Get(txCtx).ExecContext(txCtx, s.query(`
					INSERT INTO ${SCHEMA_NAME}.stages (
						id, workspace_id, slug, name, color, position, created_at, updated_at
					)
					VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				`),
					uuid.NewString(),
					workspaceID,
					seed.Slug,
					seed.Name,
					seed.Color,
					idx,
					now,
					now,
				); err != nil {
					return fmt.Errorf("insert default stage %s: %w", seed.Slug, err)
				}
			}
			return nil
		}); err != nil {
			return nil, fmt.Errorf("seed default stages: %w", err)
		}
	}

	var stages []Stage
	if err := s.db.Get(ctx).SelectContext(ctx, &stages, s.query(`
		SELECT id, workspace_id, slug, name, color, position, created_at, updated_at
		FROM ${SCHEMA_NAME}.stages
		WHERE workspace_id = ?
		ORDER BY position ASC
	`), workspaceID); err != nil {
		return nil, fmt.Errorf("list stages: %w", err)
	}
	return stages, nil
}

func (s *Store) listDeals(ctx context.Context, workspaceID string) ([]Deal, error) {
	var deals []Deal
	if err := s.db.Get(ctx).SelectContext(ctx, &deals, s.query(`
		SELECT id, workspace_id, stage_id, title, organization_name, contact_name, contact_email,
			linked_case_id, value_cents, currency, close_date, win_probability, notes, created_at, updated_at
		FROM ${SCHEMA_NAME}.deals
		WHERE workspace_id = ?
		ORDER BY updated_at DESC, created_at DESC
	`), workspaceID); err != nil {
		return nil, fmt.Errorf("list deals: %w", err)
	}
	return deals, nil
}

func (s *Store) query(input string) string {
	return strings.ReplaceAll(input, "${SCHEMA_NAME}", SchemaName)
}

type stageSeed struct {
	Slug  string
	Name  string
	Color string
}

func defaultStageSeeds(mode string) []stageSeed {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "b2c" {
		return []stageSeed{
			{Slug: "new-lead", Name: "New Lead", Color: "#94a3b8"},
			{Slug: "qualified", Name: "Qualified", Color: "#2563eb"},
			{Slug: "proposal", Name: "Proposal", Color: "#ea580c"},
			{Slug: "commit", Name: "Commit", Color: "#8b5cf6"},
			{Slug: "won", Name: "Closed Won", Color: "#16a34a"},
			{Slug: "lost", Name: "Closed Lost", Color: "#dc2626"},
		}
	}
	return []stageSeed{
		{Slug: "lead", Name: "Lead", Color: "#64748b"},
		{Slug: "qualified", Name: "Qualified", Color: "#2563eb"},
		{Slug: "proposal", Name: "Proposal", Color: "#ea580c"},
		{Slug: "negotiation", Name: "Negotiation", Color: "#8b5cf6"},
		{Slug: "won", Name: "Closed Won", Color: "#16a34a"},
		{Slug: "lost", Name: "Closed Lost", Color: "#dc2626"},
	}
}
