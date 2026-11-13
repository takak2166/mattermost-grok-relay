package webhook

// ForwardResult summarizes a single webhook POST attempt (no retries).
type ForwardResult struct {
	Success   bool
	FailClass string
}
