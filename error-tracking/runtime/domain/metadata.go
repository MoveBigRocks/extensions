package observabilitydomain

import (
	"encoding/json"
	"fmt"
)

// Metadata is extension-owned JSON metadata. It deliberately stays a plain
// map at the host boundary while retaining small convenience accessors used by
// the Sentry protocol adapter.
type Metadata map[string]any

func NewMetadata() Metadata { return Metadata{} }

func MetadataFromMap(values map[string]any) Metadata {
	result := make(Metadata, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func (m Metadata) GetString(key string) string {
	value, ok := m[key]
	if !ok || value == nil {
		return ""
	}
	if text, ok := value.(string); ok {
		return text
	}
	return fmt.Sprint(value)
}

func (m Metadata) GetInt(key string) int64 {
	switch value := m[key].(type) {
	case int:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(value)
	case json.Number:
		parsed, _ := value.Int64()
		return parsed
	default:
		return 0
	}
}

func (m Metadata) GetBool(key string) bool {
	value, _ := m[key].(bool)
	return value
}
