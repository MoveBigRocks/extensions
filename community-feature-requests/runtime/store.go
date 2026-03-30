package communityrequests

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	slugger "github.com/gosimple/slug"

	"github.com/movebigrocks/extension-sdk/extdb"
)

type Store struct {
	db *extdb.DB
}

func NewStore(db *extdb.DB) (*Store, error) {
	store := &Store{db: db}
	if err := store.ensureSchemaAvailable(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) ensureSchemaAvailable(ctx context.Context) error {
	var regclass sql.NullString
	if err := s.db.Get(ctx).GetContext(ctx, &regclass, s.query(`SELECT to_regclass(?)`), SchemaName+".feature_requests"); err != nil {
		return fmt.Errorf("check community feature requests schema availability: %w", err)
	}
	if !regclass.Valid || regclass.String == "" {
		return fmt.Errorf("community feature requests schema %s is not available", SchemaName)
	}
	return nil
}

func (s *Store) ListIdeas(ctx context.Context, workspaceID string, opts ListOptions) ([]FeatureRequest, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}

	query := `
		SELECT id, workspace_id, slug, title, description_markdown, status, is_public, vote_count,
			comment_count, submitter_name, submitter_email, linked_case_id, created_at, updated_at
		FROM ${SCHEMA_NAME}.feature_requests
		WHERE workspace_id = ?
	`
	args := []interface{}{workspaceID}

	if opts.PublicOnly {
		query += ` AND is_public = TRUE`
	}
	if status := strings.TrimSpace(opts.Status); status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	if search := strings.TrimSpace(opts.Search); search != "" {
		query += ` AND (title ILIKE ? OR description_markdown ILIKE ?)`
		term := "%" + search + "%"
		args = append(args, term, term)
	}

	switch strings.TrimSpace(opts.Sort) {
	case "new":
		query += ` ORDER BY created_at DESC, vote_count DESC`
	case "status":
		query += ` ORDER BY status ASC, vote_count DESC, created_at DESC`
	default:
		query += ` ORDER BY vote_count DESC, created_at DESC`
	}

	var ideas []FeatureRequest
	if err := s.db.Get(ctx).SelectContext(ctx, &ideas, s.query(query), args...); err != nil {
		return nil, fmt.Errorf("list ideas: %w", err)
	}
	return ideas, nil
}

func (s *Store) GetIdeaBySlug(ctx context.Context, workspaceID, slug string, publicOnly bool) (*FeatureRequest, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	slug = strings.TrimSpace(slug)
	if workspaceID == "" || slug == "" {
		return nil, fmt.Errorf("workspace ID and slug are required")
	}

	query := `
		SELECT id, workspace_id, slug, title, description_markdown, status, is_public, vote_count,
			comment_count, submitter_name, submitter_email, linked_case_id, created_at, updated_at
		FROM ${SCHEMA_NAME}.feature_requests
		WHERE workspace_id = ? AND slug = ?
	`
	args := []interface{}{workspaceID, slug}
	if publicOnly {
		query += ` AND is_public = TRUE`
	}

	idea := &FeatureRequest{}
	if err := s.db.Get(ctx).GetContext(ctx, idea, s.query(query), args...); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("idea not found")
		}
		return nil, fmt.Errorf("get idea: %w", err)
	}
	return idea, nil
}

func (s *Store) CreateIdea(ctx context.Context, input CreateIdeaInput) (*FeatureRequest, error) {
	workspaceID := strings.TrimSpace(input.WorkspaceID)
	title := strings.TrimSpace(input.Title)
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace ID is required")
	}
	if title == "" {
		return nil, fmt.Errorf("title is required")
	}

	ideaSlug, err := s.uniqueSlug(ctx, workspaceID, title)
	if err != nil {
		return nil, err
	}

	idea := &FeatureRequest{
		ID:                  uuid.NewString(),
		WorkspaceID:         workspaceID,
		Slug:                ideaSlug,
		Title:               title,
		DescriptionMarkdown: strings.TrimSpace(input.DescriptionMarkdown),
		Status:              "open",
		IsPublic:            input.IsPublic,
		VoteCount:           0,
		CommentCount:        0,
		SubmitterName:       strings.TrimSpace(input.SubmitterName),
		SubmitterEmail:      strings.TrimSpace(input.SubmitterEmail),
		LinkedCaseID:        "",
		CreatedAt:           time.Now().UTC(),
		UpdatedAt:           time.Now().UTC(),
	}

	if err := s.db.Get(ctx).GetContext(ctx, idea, s.query(`
		INSERT INTO ${SCHEMA_NAME}.feature_requests (
			id, workspace_id, slug, title, description_markdown, status, is_public, vote_count,
			comment_count, submitter_name, submitter_email, linked_case_id, created_at, updated_at
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, workspace_id, slug, title, description_markdown, status, is_public, vote_count,
			comment_count, submitter_name, submitter_email, linked_case_id, created_at, updated_at
	`),
		idea.ID,
		idea.WorkspaceID,
		idea.Slug,
		idea.Title,
		idea.DescriptionMarkdown,
		idea.Status,
		idea.IsPublic,
		idea.VoteCount,
		idea.CommentCount,
		idea.SubmitterName,
		idea.SubmitterEmail,
		idea.LinkedCaseID,
		idea.CreatedAt,
		idea.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("create idea: %w", err)
	}
	return idea, nil
}

func (s *Store) AddVote(ctx context.Context, workspaceID, slug, voterKey string) (*FeatureRequest, bool, error) {
	idea, err := s.GetIdeaBySlug(ctx, workspaceID, slug, true)
	if err != nil {
		return nil, false, err
	}
	voterKey = strings.TrimSpace(voterKey)
	if voterKey == "" {
		return nil, false, fmt.Errorf("voter key is required")
	}

	added := false
	if err := s.db.Transaction(ctx, func(txCtx context.Context) error {
		result, err := s.db.Get(txCtx).ExecContext(txCtx, s.query(`
			INSERT INTO ${SCHEMA_NAME}.feature_request_votes (
				id, workspace_id, feature_request_id, voter_key, created_at
			)
			VALUES (?, ?, ?, ?, NOW())
			ON CONFLICT (workspace_id, feature_request_id, voter_key) DO NOTHING
		`), uuid.NewString(), workspaceID, idea.ID, voterKey)
		if err != nil {
			return fmt.Errorf("insert vote: %w", err)
		}
		rowsAffected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("read vote insert status: %w", err)
		}
		if rowsAffected == 0 {
			return nil
		}
		added = true
		if _, err := s.db.Get(txCtx).ExecContext(txCtx, s.query(`
			UPDATE ${SCHEMA_NAME}.feature_requests
			SET vote_count = vote_count + 1, updated_at = NOW()
			WHERE workspace_id = ? AND id = ?
		`), workspaceID, idea.ID); err != nil {
			return fmt.Errorf("increment vote count: %w", err)
		}
		return nil
	}); err != nil {
		return nil, false, err
	}

	updated, err := s.GetIdeaBySlug(ctx, workspaceID, slug, true)
	if err != nil {
		return nil, added, err
	}
	return updated, added, nil
}

func (s *Store) UpdateIdea(ctx context.Context, workspaceID, ideaID string, input UpdateIdeaInput) (*FeatureRequest, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	ideaID = strings.TrimSpace(ideaID)
	if workspaceID == "" || ideaID == "" {
		return nil, fmt.Errorf("workspace ID and idea ID are required")
	}
	if !isValidStatus(input.Status) {
		return nil, fmt.Errorf("invalid status %q", input.Status)
	}

	idea := &FeatureRequest{}
	if err := s.db.Get(ctx).GetContext(ctx, idea, s.query(`
		UPDATE ${SCHEMA_NAME}.feature_requests
		SET status = ?, is_public = ?, updated_at = NOW()
		WHERE workspace_id = ? AND id = ?
		RETURNING id, workspace_id, slug, title, description_markdown, status, is_public, vote_count,
			comment_count, submitter_name, submitter_email, linked_case_id, created_at, updated_at
	`), input.Status, input.IsPublic, workspaceID, ideaID); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("idea not found")
		}
		return nil, fmt.Errorf("update idea: %w", err)
	}
	return idea, nil
}

func (s *Store) uniqueSlug(ctx context.Context, workspaceID, title string) (string, error) {
	base := strings.TrimSpace(slugger.Make(title))
	if base == "" {
		base = "idea"
	}
	candidate := base
	for attempt := 0; attempt < 100; attempt++ {
		var count int
		if err := s.db.Get(ctx).GetContext(ctx, &count, s.query(`
			SELECT COUNT(*) FROM ${SCHEMA_NAME}.feature_requests WHERE workspace_id = ? AND slug = ?
		`), workspaceID, candidate); err != nil {
			return "", fmt.Errorf("check idea slug: %w", err)
		}
		if count == 0 {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d", base, attempt+2)
	}
	return "", fmt.Errorf("could not allocate a unique slug")
}

func (s *Store) query(input string) string {
	return strings.ReplaceAll(input, "${SCHEMA_NAME}", SchemaName)
}

func isValidStatus(status string) bool {
	status = strings.TrimSpace(status)
	for _, valid := range ValidStatuses {
		if status == valid {
			return true
		}
	}
	return false
}
