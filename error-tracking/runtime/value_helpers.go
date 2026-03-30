package sql

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
