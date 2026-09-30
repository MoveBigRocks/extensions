// Package domain owns the operational signal and freshness rules.
package domain

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

const ContractVersion = "operational-health/v1"
const FreshFor = 180 * time.Second

var ErrInvalid = errors.New("invalid operational health input")
var ErrForbidden = errors.New("source credential or scope is invalid")
var ErrNotFound = errors.New("operational record not found")
var ErrConflict = errors.New("operational revision or request key conflict")
var slug = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,79}$`)

type Config struct {
	QueueID   string   `json:"queueId"`
	QueueSlug string   `json:"queueSlug"`
	Sources   []Source `json:"sources"`
}
type Source struct {
	ID          string               `json:"id"`
	Environment string               `json:"environment"`
	Enabled     bool                 `json:"enabled"`
	Canary      bool                 `json:"canary"`
	TokenHashes []string             `json:"tokenHashes"`
	Components  map[string]Component `json:"components"`
	Alerts      map[string]string    `json:"alerts"`
}
type Component struct {
	Checks        []string `json:"checks"`
	RunbookID     string   `json:"runbookId,omitempty"`
	OwnerID       string   `json:"ownerId,omitempty"`
	CatalogNodeID string   `json:"catalogNodeId,omitempty"`
}

func (c Config) Validate() error {
	if !slug.MatchString(c.QueueSlug) || len(c.Sources) > 32 {
		return ErrInvalid
	}
	if len(c.Sources) > 0 {
		if _, err := uuid.Parse(c.QueueID); err != nil {
			return ErrInvalid
		}
	}
	seen := map[string]bool{}
	for _, s := range c.Sources {
		if _, err := uuid.Parse(s.ID); err != nil {
			return ErrInvalid
		}
		if seen[s.ID] {
			return ErrInvalid
		}
		seen[s.ID] = true
		if !slug.MatchString(s.Environment) || len(s.Components) == 0 || len(s.Components) > 32 || len(s.TokenHashes) > 2 || (s.Enabled && len(s.TokenHashes) == 0) {
			return ErrInvalid
		}
		for _, h := range s.TokenHashes {
			b, err := hex.DecodeString(h)
			if err != nil || len(b) != 32 {
				return ErrInvalid
			}
		}
		for name, target := range s.Components {
			if !slug.MatchString(name) || len(target.Checks) == 0 || len(target.Checks) > 32 {
				return ErrInvalid
			}
			checks := map[string]bool{}
			for _, check := range target.Checks {
				if !slug.MatchString(check) || checks[check] {
					return ErrInvalid
				}
				checks[check] = true
			}
			for _, id := range []string{target.RunbookID, target.OwnerID, target.CatalogNodeID} {
				if id != "" {
					if _, err := uuid.Parse(id); err != nil {
						return ErrInvalid
					}
				}
			}
		}
		for alert, component := range s.Alerts {
			if len(alert) == 0 || len(alert) > 100 || s.Components[component].Checks == nil {
				return ErrInvalid
			}
		}
	}
	return nil
}
func (c Config) Authenticate(id, token string) (Source, error) {
	if len(token) < 32 || len(token) > 512 || strings.HasPrefix(token, "hat_") {
		return Source{}, ErrForbidden
	}
	sum := sha256.Sum256([]byte(token))
	matched := 0
	for _, s := range c.Sources {
		if s.ID != id || !s.Enabled {
			continue
		}
		for _, hash := range s.TokenHashes {
			expected, err := hex.DecodeString(hash)
			if err == nil {
				matched |= subtle.ConstantTimeCompare(sum[:], expected)
			}
		}
		if matched == 1 {
			return s, nil
		}
	}
	return Source{}, ErrForbidden
}

type AlertBatch struct {
	Version   string  `json:"version"`
	Truncated int     `json:"truncatedAlerts"`
	Alerts    []Alert `json:"alerts"`
}
type Alert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
	Fingerprint string            `json:"fingerprint"`
}
type Signal struct {
	Component   string     `json:"component"`
	AlertName   string     `json:"alertName"`
	Severity    string     `json:"severity"`
	Fingerprint string     `json:"fingerprint"`
	StartsAt    time.Time  `json:"startsAt"`
	EndsAt      *time.Time `json:"endsAt,omitempty"`
	Status      string     `json:"status"`
}

func Normalize(source Source, batch AlertBatch, now time.Time) ([]Signal, bool, error) {
	if batch.Version != "4" || len(batch.Alerts) == 0 || len(batch.Alerts) > 100 || batch.Truncated < 0 {
		return nil, false, ErrInvalid
	}
	out := make([]Signal, 0, len(batch.Alerts))
	canary := false
	for _, a := range batch.Alerts {
		name := a.Labels["alertname"]
		if env := a.Labels["environment"]; env != "" && env != source.Environment {
			return nil, false, ErrForbidden
		}
		if a.Status != "firing" && a.Status != "resolved" {
			return nil, false, ErrInvalid
		}
		if a.StartsAt.IsZero() || a.StartsAt.After(now.Add(30*time.Second)) || len(a.Fingerprint) < 8 || len(a.Fingerprint) > 64 {
			return nil, false, ErrInvalid
		}
		if _, err := hex.DecodeString(a.Fingerprint); err != nil {
			return nil, false, ErrInvalid
		}
		if name == "MBRDeliveryCanary" {
			if a.Status != "firing" {
				return nil, false, ErrInvalid
			}
			canary = true
			continue
		}
		component, ok := source.Alerts[name]
		if !ok {
			return nil, false, fmt.Errorf("%w: unmapped alert", ErrInvalid)
		}
		severity := a.Labels["severity"]
		if severity != "critical" && severity != "warning" && severity != "info" {
			severity = "warning"
		}
		signal := Signal{Component: component, AlertName: name, Severity: severity, Fingerprint: a.Fingerprint, StartsAt: a.StartsAt.UTC(), Status: a.Status}
		if a.Status == "resolved" {
			if a.EndsAt.IsZero() || a.EndsAt.Before(a.StartsAt) || a.EndsAt.After(now.Add(30*time.Second)) {
				return nil, false, ErrInvalid
			}
			end := a.EndsAt.UTC()
			signal.EndsAt = &end
		}
		out = append(out, signal)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Fingerprint != out[j].Fingerprint {
			return out[i].Fingerprint < out[j].Fingerprint
		}
		return out[i].StartsAt.Before(out[j].StartsAt)
	})
	return out, canary, nil
}
func Digest(value any) string {
	data, _ := json.Marshal(value)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
func Transition(previous, incoming string) string {
	if previous == "resolved" {
		return "resolved"
	}
	return incoming
}

type Check struct {
	Name  string `json:"name"`
	State string `json:"state"`
}
type Observation struct {
	Component  string    `json:"component"`
	ObservedAt time.Time `json:"observedAt"`
	Checks     []Check   `json:"checks"`
	Release    string    `json:"release,omitempty"`
	Slot       string    `json:"slot,omitempty"`
}

func (o Observation) Validate(source Source, now time.Time) error {
	target, ok := source.Components[o.Component]
	if !ok || o.ObservedAt.IsZero() || o.ObservedAt.After(now.Add(30*time.Second)) || len(o.Checks) > 32 || len(o.Release) > 128 || (o.Slot != "" && o.Slot != "blue" && o.Slot != "green") {
		return ErrInvalid
	}
	allowed := map[string]bool{}
	for _, check := range target.Checks {
		allowed[check] = true
	}
	seen := map[string]bool{}
	for _, check := range o.Checks {
		if !allowed[check.Name] || seen[check.Name] || (check.State != "healthy" && check.State != "unhealthy" && check.State != "unknown") {
			return ErrInvalid
		}
		seen[check.Name] = true
	}
	if o.Release != "" && !regexp.MustCompile(`^[a-zA-Z0-9._-]+$`).MatchString(o.Release) {
		return ErrInvalid
	}
	return nil
}
func (o Observation) State(target Component, now time.Time) string {
	if o.ObservedAt.IsZero() || now.Sub(o.ObservedAt) > FreshFor || o.ObservedAt.After(now.Add(30*time.Second)) {
		return "unknown"
	}
	found := map[string]string{}
	for _, check := range o.Checks {
		found[check.Name] = check.State
	}
	state := "healthy"
	for _, check := range target.Checks {
		if found[check] == "unhealthy" {
			return "unhealthy"
		}
		if found[check] != "healthy" {
			state = "unknown"
		}
	}
	return state
}

type Incident struct {
	ID          string     `json:"id"`
	WorkspaceID string     `json:"workspaceId"`
	SourceID    string     `json:"sourceId"`
	Environment string     `json:"environment"`
	Component   string     `json:"component"`
	Severity    string     `json:"severity"`
	CaseID      string     `json:"caseId,omitempty"`
	CaseStatus  string     `json:"caseStatus,omitempty"`
	SignalState string     `json:"signalState"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	ClosedAt    *time.Time `json:"closedAt,omitempty"`
	Revision    int64      `json:"revision,string"`
	OwnerID     string     `json:"ownerId,omitempty"`
	RunbookID   string     `json:"runbookId,omitempty"`
	Canary      bool       `json:"canary"`
}
type Page struct {
	Items []Incident `json:"items"`
	Next  string     `json:"next,omitempty"`
}
type TargetStatus struct {
	SourceID           string       `json:"sourceId"`
	Environment        string       `json:"environment"`
	Component          string       `json:"component"`
	State              string       `json:"state"`
	Observation        *Observation `json:"observation,omitempty"`
	ExpiresAt          *time.Time   `json:"expiresAt,omitempty"`
	LastDelivery       *time.Time   `json:"lastDelivery,omitempty"`
	LastSnapshot       *time.Time   `json:"lastSnapshot,omitempty"`
	LastCanary         *time.Time   `json:"lastCanary,omitempty"`
	DeliveryState      string       `json:"deliveryState"`
	ActiveSignals      int          `json:"activeSignals"`
	PendingProjections int          `json:"pendingProjections"`
	MaintenanceUntil   *time.Time   `json:"maintenanceUntil,omitempty"`
	DeploymentState    string       `json:"deploymentState"`
	DeploymentRunID    string       `json:"deploymentRunId,omitempty"`
	OwnerID            string       `json:"ownerId,omitempty"`
	RunbookID          string       `json:"runbookId,omitempty"`
	Canary             bool         `json:"canary"`
}
type Deployment struct {
	RunID     string    `json:"runId"`
	Component string    `json:"component"`
	StartsAt  time.Time `json:"startsAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	State     string    `json:"state"`
	Release   string    `json:"release,omitempty"`
}

func (d Deployment) Validate(source Source, now time.Time) error {
	if !regexp.MustCompile(`^[a-zA-Z0-9._-]{1,128}$`).MatchString(d.RunID) || source.Components[d.Component].Checks == nil || d.StartsAt.IsZero() || d.StartsAt.After(now.Add(30*time.Second)) || d.ExpiresAt.Before(d.StartsAt) || d.ExpiresAt.Sub(d.StartsAt) > 30*time.Minute || (d.State != "started" && d.State != "completed" && d.State != "failed") || len(d.Release) > 128 {
		return ErrInvalid
	}
	return nil
}

type Filter struct {
	Environment string
	Component   string
	State       string
	Severity    string
	Owner       string
	After       string
	Limit       int
}
