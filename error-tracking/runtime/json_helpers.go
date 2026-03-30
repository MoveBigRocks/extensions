package sql

import (
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"

	shareddomain "github.com/movebigrocks/platform/pkg/extensionhost/shared/domain"
	"github.com/movebigrocks/platform/pkg/logger"
)

func marshalJSONString(v interface{}, fieldName string) (string, error) {
	if v == nil {
		return "", nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("marshal %s: %w", fieldName, err)
	}
	return string(data), nil
}

func unmarshalJSONField(jsonStr string, target interface{}, table, field string) {
	if jsonStr == "" {
		return
	}
	if err := json.Unmarshal([]byte(jsonStr), target); err != nil {
		logger.New().Warn("Failed to unmarshal JSON field", "table", table, "field", field, "error", err)
	}
}

func unmarshalJSONBytes(data []byte, target interface{}, table, field string) {
	if len(data) == 0 {
		return
	}
	if err := json.Unmarshal(data, target); err != nil {
		logger.New().Warn("Failed to unmarshal JSON bytes", "table", table, "field", field, "error", err)
	}
}

func unmarshalMetadataOrEmpty(jsonStr string, table, field string) shareddomain.Metadata {
	if jsonStr == "" {
		return shareddomain.NewMetadata()
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		logger.New().Warn("Failed to unmarshal metadata", "table", table, "field", field, "error", err)
		return shareddomain.NewMetadata()
	}
	return shareddomain.MetadataFromMap(m)
}

func buildInQuery(query string, args interface{}) (string, []interface{}, error) {
	q, qArgs, err := sqlx.In(query, args)
	if err != nil {
		return "", nil, fmt.Errorf("build IN query: %w", err)
	}
	return q, qArgs, nil
}
