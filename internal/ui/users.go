package ui

import (
	"strings"

	"yt-tui/internal/ytcli"
)

func filterUsersByQuery(users []ytcli.User, query string, limit int) []ytcli.User {
	if len(users) == 0 {
		return nil
	}
	if limit <= 0 {
		limit = 8
	}
	q := strings.ToLower(query)
	if q == "" {
		result := make([]ytcli.User, len(users))
		copy(result, users)
		if len(result) > limit {
			result = result[:limit]
		}
		return result
	}
	var result []ytcli.User
	for _, u := range users {
		if strings.Contains(strings.ToLower(u.FullName), q) ||
			strings.Contains(strings.ToLower(u.Login), q) {
			result = append(result, u)
			if len(result) >= limit {
				break
			}
		}
	}
	return result
}
