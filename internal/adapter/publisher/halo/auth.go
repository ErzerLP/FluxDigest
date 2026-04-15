package halo

import (
	"net/http"
	"strings"
)

const basicTokenPrefix = "basic:"

// ApplyAuthorizationHeader 按配置格式写入 Halo 鉴权头。
func ApplyAuthorizationHeader(req *http.Request, token string) {
	if req == nil {
		return
	}

	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		return
	}

	if encoded, ok := parseBasicToken(trimmed); ok {
		req.Header.Set("Authorization", "Basic "+encoded)
		return
	}

	req.Header.Set("Authorization", "Bearer "+trimmed)
}

func parseBasicToken(token string) (string, bool) {
	if !strings.HasPrefix(strings.ToLower(token), basicTokenPrefix) {
		return "", false
	}

	encoded := strings.TrimSpace(token[len(basicTokenPrefix):])
	if encoded == "" {
		return "", false
	}

	return encoded, true
}
