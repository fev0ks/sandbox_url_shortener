package domain

import "time"

// VisitMetadata is the transport-derived request context needed to publish a
// versioned link.visited event after a successful redirect.
type VisitMetadata struct {
	RequestID  string
	OccurredAt time.Time
	IP         string
	UserAgent  string
	Referer    string
}
