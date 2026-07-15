package sql

func derefStringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
