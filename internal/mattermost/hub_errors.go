package mattermost

import "errors"

// ErrAllListenersStopped is returned when every bot listener exits while the hub context is still active.
var ErrAllListenersStopped = errors.New("mattermost: all bot listeners stopped")
