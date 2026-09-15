package sql

import (
	"context"
	"fmt"
	"strings"
	"time"

	observabilitydomain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
	models "github.com/movebigrocks/extensions/error-tracking/sql-models"
)

// issueCaseClaimLease is how long an unfinished claim blocks a redelivery before
// another attempt may take it over. A claim is completed with the case id in the
// call right after the case is created, so an unfinished claim older than this
// belongs to a runtime that died mid-flight; without the lease that claim would
// block every later redelivery of the event forever.
const issueCaseClaimLease = 15 * time.Minute

const issueCaseClaimColumns = `workspace_id, extension_install_id, issue_id, dedup_key, case_id, claimed_at, created_at, updated_at`

// GetIssueCaseClaim returns the claim recorded for an issue and dedup key. A
// claim with an empty CaseID has been taken but not yet completed.
func (s *ErrorMonitoringStore) GetIssueCaseClaim(ctx context.Context, workspaceID, issueID, dedupKey string) (*observabilitydomain.IssueCaseClaim, error) {
	var model models.IssueCaseClaim
	query := `SELECT ` + issueCaseClaimColumns + ` FROM ${SCHEMA_NAME}.issue_case_claims
		WHERE workspace_id = ? AND issue_id = ? AND dedup_key = ?`
	if err := s.getContext(ctx, &model, query, strings.TrimSpace(workspaceID), strings.TrimSpace(issueID), dedupKey); err != nil {
		return nil, TranslateSqlxError(err, "issue_case_claims")
	}
	return mapIssueCaseClaimToDomain(&model), nil
}

// ClaimIssueCase takes the (issue, dedup key) claim before any core case is
// created, so a redelivery of the same event cannot produce a second one. It
// reports whether this caller now holds the claim; a caller that does not reads
// the stored claim to see whether the case already exists or another attempt is
// still in flight.
func (s *ErrorMonitoringStore) ClaimIssueCase(ctx context.Context, workspaceID, issueID, dedupKey string) (bool, error) {
	// The insert draws the workspace and install from the issue row itself, so a
	// claim can only be taken for an issue that really is in the named workspace.
	query := `
		INSERT INTO ${SCHEMA_NAME}.issue_case_claims (
			workspace_id, extension_install_id, issue_id, dedup_key, claimed_at, created_at, updated_at
		)
		SELECT i.workspace_id, i.extension_install_id, i.id, ?, NOW(), NOW(), NOW()
		FROM ${SCHEMA_NAME}.issues i
		WHERE i.id = ? AND i.workspace_id = ?
		ON CONFLICT (workspace_id, issue_id, dedup_key) DO UPDATE
			SET claimed_at = NOW(), updated_at = NOW()
			WHERE issue_case_claims.case_id IS NULL
				AND issue_case_claims.claimed_at < NOW() - ?::interval`
	result, err := s.execContext(ctx, query,
		dedupKey,
		strings.TrimSpace(issueID),
		strings.TrimSpace(workspaceID),
		fmt.Sprintf("%d seconds", int(issueCaseClaimLease.Seconds())),
	)
	if err != nil {
		return false, TranslateSqlxError(err, "issue_case_claims")
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return rows > 0, nil
}

// CompleteIssueCaseClaim records the case the claim produced, which is what
// later redeliveries reuse instead of creating another case.
func (s *ErrorMonitoringStore) CompleteIssueCaseClaim(ctx context.Context, workspaceID, issueID, dedupKey, caseID string) error {
	query := `UPDATE ${SCHEMA_NAME}.issue_case_claims SET case_id = ?, updated_at = NOW()
		WHERE workspace_id = ? AND issue_id = ? AND dedup_key = ?`
	result, err := s.execContext(ctx, query, strings.TrimSpace(caseID), strings.TrimSpace(workspaceID), strings.TrimSpace(issueID), dedupKey)
	if err != nil {
		return TranslateSqlxError(err, "issue_case_claims")
	}
	rows, rowsErr := result.RowsAffected()
	if rowsErr == nil && rows == 0 {
		return ErrNotFound
	}
	return nil
}

// ReleaseIssueCaseClaim drops an unfinished claim so the next delivery of the
// event retries immediately instead of waiting out the lease. It never drops a
// completed claim, which would let the case be created a second time.
func (s *ErrorMonitoringStore) ReleaseIssueCaseClaim(ctx context.Context, workspaceID, issueID, dedupKey string) error {
	query := `DELETE FROM ${SCHEMA_NAME}.issue_case_claims
		WHERE workspace_id = ? AND issue_id = ? AND dedup_key = ? AND case_id IS NULL`
	if _, err := s.execContext(ctx, query, strings.TrimSpace(workspaceID), strings.TrimSpace(issueID), dedupKey); err != nil {
		return TranslateSqlxError(err, "issue_case_claims")
	}
	return nil
}

func mapIssueCaseClaimToDomain(m *models.IssueCaseClaim) *observabilitydomain.IssueCaseClaim {
	claim := &observabilitydomain.IssueCaseClaim{
		WorkspaceID: m.WorkspaceID,
		IssueID:     m.IssueID,
		DedupKey:    m.DedupKey,
		ClaimedAt:   m.ClaimedAt,
	}
	if m.CaseID != nil {
		claim.CaseID = *m.CaseID
	}
	return claim
}
