package sql

import (
	"encoding/json"

	coremodels "github.com/movebigrocks/platform/pkg/extensionhost/infrastructure/stores/sql/models"
	servicedomain "github.com/movebigrocks/platform/pkg/extensionhost/service/domain"
	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
)

func mapStoredCaseToDomain(c *coremodels.Case) *servicedomain.Case {
	var tags []string
	if err := json.Unmarshal([]byte(c.Tags), &tags); err != nil {
		tags = []string{}
	}

	var linkedIssues []string
	if err := json.Unmarshal([]byte(c.LinkedIssueIDs), &linkedIssues); err != nil {
		linkedIssues = []string{}
	}

	customFields := shareddomain.NewTypedCustomFields()
	if c.CustomFields != "" {
		unmarshalJSONField(c.CustomFields, &customFields, "cases", "custom_fields")
	}

	return &servicedomain.Case{
		CaseIdentity: servicedomain.CaseIdentity{
			ID:          c.ID,
			WorkspaceID: c.WorkspaceID,
			HumanID:     c.HumanID,
		},
		Subject:              c.Subject,
		Description:          c.Description,
		Status:               servicedomain.CaseStatus(c.Status),
		Priority:             servicedomain.CasePriority(c.Priority),
		Channel:              servicedomain.CaseChannel(c.Channel),
		Category:             c.Category,
		QueueID:              valueOrEmpty(c.QueueID),
		PrimaryCatalogNodeID: valueOrEmpty(c.PrimaryCatalogNodeID),
		Tags:                 tags,
		CaseContact: servicedomain.CaseContact{
			ContactID:    derefStringPtr(c.ContactID),
			ContactEmail: c.ContactEmail,
			ContactName:  c.ContactName,
		},
		CaseAssignment: servicedomain.CaseAssignment{
			AssignedToID: derefStringPtr(c.AssignedToID),
			TeamID:       derefStringPtr(c.TeamID),
		},
		CaseSourceInfo: servicedomain.CaseSourceInfo{
			Source:       shareddomain.SourceType(c.Source),
			AutoCreated:  c.AutoCreated,
			IsSystemCase: c.IsSystemCase,
		},
		CaseSLA: servicedomain.CaseSLA{
			ResponseDueAt:         c.ResponseDueAt,
			ResolutionDueAt:       c.ResolutionDueAt,
			FirstResponseAt:       c.FirstResponseAt,
			ResolvedAt:            c.ResolvedAt,
			ClosedAt:              c.ClosedAt,
			ResponseTimeMinutes:   c.ResponseTimeMinutes,
			ResolutionTimeMinutes: c.ResolutionTimeMinutes,
		},
		CaseTimestamps: servicedomain.CaseTimestamps{
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		},
		CaseMetrics: servicedomain.CaseMetrics{
			ReopenCount:  c.ReopenCount,
			MessageCount: c.MessageCount,
		},
		CaseRelationships: servicedomain.CaseRelationships{},
		CaseIssueTracking: servicedomain.CaseIssueTracking{
			LinkedIssueIDs:       linkedIssues,
			RootCauseIssueID:     derefStringPtr(c.RootCauseIssueID),
			IssueResolved:        c.IssueResolved,
			IssueResolvedAt:      c.IssueResolvedAt,
			ContactNotified:      c.ContactNotified,
			ContactNotifiedAt:    c.ContactNotifiedAt,
			NotificationTemplate: c.NotificationTemplate,
		},
		OriginatingConversationID: valueOrEmpty(c.OriginatingConversationID),
		CustomFields:              customFields,
	}
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func derefStringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
