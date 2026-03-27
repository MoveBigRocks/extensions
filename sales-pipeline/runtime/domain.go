package salespipeline

import "time"

const SchemaName = "ext_demandops_sales_pipeline"

type Stage struct {
	ID        string    `db:"id" json:"id"`
	Workspace string    `db:"workspace_id" json:"-"`
	Slug      string    `db:"slug" json:"slug"`
	Name      string    `db:"name" json:"name"`
	Color     string    `db:"color" json:"color"`
	Position  int       `db:"position" json:"position"`
	CreatedAt time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt time.Time `db:"updated_at" json:"updatedAt"`
}

type Deal struct {
	ID               string     `db:"id" json:"id"`
	Workspace        string     `db:"workspace_id" json:"-"`
	StageID          string     `db:"stage_id" json:"stageId"`
	Title            string     `db:"title" json:"title"`
	OrganizationName string     `db:"organization_name" json:"organizationName"`
	ContactName      string     `db:"contact_name" json:"contactName"`
	ContactEmail     string     `db:"contact_email" json:"contactEmail"`
	LinkedCaseID     string     `db:"linked_case_id" json:"linkedCaseId"`
	ValueCents       int64      `db:"value_cents" json:"valueCents"`
	Currency         string     `db:"currency" json:"currency"`
	CloseDate        *time.Time `db:"close_date" json:"closeDate,omitempty"`
	WinProbability   int        `db:"win_probability" json:"winProbability"`
	Notes            string     `db:"notes" json:"notes"`
	CreatedAt        time.Time  `db:"created_at" json:"createdAt"`
	UpdatedAt        time.Time  `db:"updated_at" json:"updatedAt"`
}

type StageColumn struct {
	Stage      Stage  `json:"stage"`
	Deals      []Deal `json:"deals"`
	TotalCents int64  `json:"totalCents"`
	TotalCount int    `json:"totalCount"`
}

type BoardSummary struct {
	TotalDeals int   `json:"totalDeals"`
	TotalCents int64 `json:"totalCents"`
}

type Board struct {
	Mode    string        `json:"mode"`
	Summary BoardSummary  `json:"summary"`
	Stages  []StageColumn `json:"stages"`
}

type CreateDealInput struct {
	WorkspaceID      string
	Title            string
	OrganizationName string
	ContactName      string
	ContactEmail     string
	LinkedCaseID     string
	ValueCents       int64
	Currency         string
	CloseDate        *time.Time
	WinProbability   int
	Notes            string
}
