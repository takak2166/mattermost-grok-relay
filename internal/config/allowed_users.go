package config

import (
	"fmt"
	"strings"
)

const envAllowedUserIDs = "RELAY_ALLOWED_USER_IDS"

// AllowedUserSet is the set of Mattermost user IDs permitted to trigger relay handling.
type AllowedUserSet map[string]struct{}

// Contains reports whether userID is in the allowlist.
func (s AllowedUserSet) Contains(userID string) bool {
	if s == nil {
		return false
	}
	_, ok := s[userID]
	return ok
}

// AllowedUserIDs loads RELAY_ALLOWED_USER_IDS (comma-separated user IDs, trim whitespace, ignore empty elements).
// Missing, unset, or empty-after-parse values fail closed so the process must not start without an allowlist.
func AllowedUserIDs(lookup func(string) (string, bool)) (AllowedUserSet, error) {
	raw, ok := lookup(envAllowedUserIDs)
	if !ok {
		return nil, fmt.Errorf("config: %s is required", envAllowedUserIDs)
	}
	ids := ParseCommaSeparatedIDs(raw)
	if len(ids) == 0 {
		return nil, fmt.Errorf("config: %s must list at least one user id", envAllowedUserIDs)
	}
	set := make(AllowedUserSet, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set, nil
}

// ParseCommaSeparatedIDs splits on commas, trims each segment, and drops empty segments.
func ParseCommaSeparatedIDs(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" {
			continue
		}
		out = append(out, id)
	}
	return out
}
