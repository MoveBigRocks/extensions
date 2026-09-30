package domain

import "time"

// Snapshot is a complete, bounded read of the source Alertmanager API. Absence
// records uncertainty about a past transition, never an invented resolved event.
type Snapshot struct {
	ObservedAt time.Time `json:"observedAt"`
	Complete   bool      `json:"complete"`
	Alerts     []Alert   `json:"alerts"`
}

func (s Snapshot) Normalize(src Source, now time.Time) ([]Signal, error) {
	if !s.Complete || s.ObservedAt.IsZero() || s.ObservedAt.After(now.Add(30*time.Second)) || now.Sub(s.ObservedAt) > FreshFor || len(s.Alerts) > 1000 {
		return nil, ErrInvalid
	}
	result := []Signal{}
	for start := 0; start < len(s.Alerts); start += 100 {
		end := min(start+100, len(s.Alerts))
		normalized, _, err := Normalize(src, AlertBatch{Version: "4", Alerts: s.Alerts[start:end]}, now)
		if err != nil {
			return nil, err
		}
		result = append(result, normalized...)
	}
	return result, nil
}
