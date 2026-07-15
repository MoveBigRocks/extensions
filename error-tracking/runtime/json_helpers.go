package sql

import (
	"encoding/json"
	"fmt"

	"github.com/jmoiron/sqlx"

	"github.com/movebigrocks/extension-sdk/logger"
	observabilitydomain "github.com/movebigrocks/extensions/error-tracking/runtime/domain"
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

func unmarshalMetadataOrEmpty(jsonStr string, table, field string) observabilitydomain.Metadata {
	if jsonStr == "" {
		return observabilitydomain.NewMetadata()
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &m); err != nil {
		logger.New().Warn("Failed to unmarshal metadata", "table", table, "field", field, "error", err)
		return observabilitydomain.NewMetadata()
	}
	return observabilitydomain.MetadataFromMap(m)
}

func buildInQuery(query string, args interface{}) (string, []interface{}, error) {
	q, qArgs, err := sqlx.In(query, args)
	if err != nil {
		return "", nil, fmt.Errorf("build IN query: %w", err)
	}
	return q, qArgs, nil
}
