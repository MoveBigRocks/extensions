package communityrequests

import "time"

const SchemaName = "ext_demandops_community_feature_requests"

var ValidStatuses = []string{
	"open",
	"planned",
	"in-progress",
	"shipped",
	"declined",
}

type FeatureRequest struct {
	ID                  string    `db:"id"`
	WorkspaceID         string    `db:"workspace_id"`
	Slug                string    `db:"slug"`
	Title               string    `db:"title"`
	DescriptionMarkdown string    `db:"description_markdown"`
	Status              string    `db:"status"`
	IsPublic            bool      `db:"is_public"`
	VoteCount           int       `db:"vote_count"`
	CommentCount        int       `db:"comment_count"`
	SubmitterName       string    `db:"submitter_name"`
	SubmitterEmail      string    `db:"submitter_email"`
	LinkedCaseID        string    `db:"linked_case_id"`
	CreatedAt           time.Time `db:"created_at"`
	UpdatedAt           time.Time `db:"updated_at"`
}

type ListOptions struct {
	PublicOnly bool
	Status     string
	Search     string
	Sort       string
}

type CreateIdeaInput struct {
	WorkspaceID         string
	Title               string
	DescriptionMarkdown string
	SubmitterName       string
	SubmitterEmail      string
	IsPublic            bool
}

type UpdateIdeaInput struct {
	Status   string
	IsPublic bool
}
