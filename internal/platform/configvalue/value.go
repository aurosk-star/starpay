package configvalue

import "strings"

func BoolDefault(value any, fallback bool) bool {
	switch typed := value.(type) {
	case nil:
		return fallback
	case bool:
		return typed
	case string:
		trimmed := strings.ToLower(strings.TrimSpace(typed))
		if trimmed == "" {
			return fallback
		}
		return trimmed == "true" || trimmed == "1" || trimmed == "yes" || trimmed == "on"
	default:
		return fallback
	}
}
