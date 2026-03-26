package sql

import (
	"os"
	"strings"

	"github.com/google/uuid"
)

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

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
