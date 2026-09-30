package domain

import "time"

type EvidenceFilter struct {
	Limit                          int
	SignalsAfter, ProjectionsAfter string
}
type SignalEvidence struct {
	ID string `json:"id"`
	Signal
	ReceivedAt time.Time `json:"receivedAt"`
}
type SignalPage struct {
	Items []SignalEvidence `json:"items"`
	Next  string           `json:"next,omitempty"`
}
type ProjectionEvidence struct {
	ID          string     `json:"id"`
	Kind        string     `json:"kind"`
	State       string     `json:"state"`
	Attempts    int        `json:"attempts"`
	LastError   string     `json:"lastError,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
	RetryAt     time.Time  `json:"retryAt"`
	DeliveredAt *time.Time `json:"deliveredAt,omitempty"`
}
type ProjectionPage struct {
	Items []ProjectionEvidence `json:"items"`
	Next  string               `json:"next,omitempty"`
}
type Evidence struct {
	Signals     SignalPage     `json:"signals"`
	Projections ProjectionPage `json:"projections"`
}
type Detail struct {
	Incident Incident      `json:"incident"`
	Evidence Evidence      `json:"evidence"`
	Target   *TargetStatus `json:"target,omitempty"`
}
