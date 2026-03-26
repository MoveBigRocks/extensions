package sql

import (
	"strings"

	"github.com/google/uuid"
)

func normalizePersistedUUID(id *string) {
	if id == nil {
		return
	}

	value := strings.TrimSpace(*id)
	if value == "" {
		*id = ""
		return
	}

	parsed, err := uuid.Parse(value)
	if err != nil {
		*id = ""
		return
	}

	*id = parsed.String()
}

func nullableUUIDValue(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	parsed, err := uuid.Parse(value)
	if err != nil {
		return value
	}

	return parsed.String()
}

func nullableLegacyUUIDValue(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}

	parsed, err := uuid.Parse(value)
	if err != nil {
		return nil
	}

	return parsed.String()
}
