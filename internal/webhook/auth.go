package webhook

import "net/http"

// SetRequestAuth attaches outbound webhook credentials to req.
// The header format is not finalized; only this function should change when it is.
func SetRequestAuth(req *http.Request, key string) {
	req.Header.Set("Authorization", "Bearer "+key)
}
