package eventing

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	LinkVisitedEventName = "link.visited"
	LinkVisitedVersion   = "v1"
)

var ErrInvalidLinkVisited = errors.New("invalid link.visited event")

// LinkVisited is the shared versioned contract used by the shortener producer
// and analytics consumer. It intentionally carries only request-derived visit
// metadata; storage-side enrichment such as country stays out of the Kafka
// payload for now.
type LinkVisited struct {
	EventName  string    `json:"event_name"`
	Version    string    `json:"version"`
	EventID    string    `json:"event_id"`
	RequestID  string    `json:"request_id"`
	LinkID     string    `json:"link_id"`
	ShortCode  string    `json:"short_code"`
	OccurredAt time.Time `json:"occurred_at"`
	IP         string    `json:"ip"`
	UserAgent  string    `json:"user_agent,omitempty"`
	Referer    string    `json:"referer,omitempty"`
}

type LinkVisitedParams struct {
	EventID    string
	RequestID  string
	LinkID     string
	ShortCode  string
	OccurredAt time.Time
	IP         string
	UserAgent  string
	Referer    string
}

func NewLinkVisited(params LinkVisitedParams) (*LinkVisited, error) {
	event := &LinkVisited{
		EventName:  LinkVisitedEventName,
		Version:    LinkVisitedVersion,
		EventID:    params.EventID,
		RequestID:  params.RequestID,
		LinkID:     params.LinkID,
		ShortCode:  params.ShortCode,
		OccurredAt: params.OccurredAt.UTC(),
		IP:         params.IP,
		UserAgent:  params.UserAgent,
		Referer:    params.Referer,
	}

	if err := event.Validate(); err != nil {
		return nil, err
	}

	return event, nil
}

func (e *LinkVisited) Validate() error {
	if e == nil {
		return fmt.Errorf("%w: event is nil", ErrInvalidLinkVisited)
	}
	if e.EventName != LinkVisitedEventName {
		return fmt.Errorf("%w: event_name must be %q", ErrInvalidLinkVisited, LinkVisitedEventName)
	}
	if e.Version != LinkVisitedVersion {
		return fmt.Errorf("%w: version must be %q", ErrInvalidLinkVisited, LinkVisitedVersion)
	}
	if _, err := uuid.Parse(strings.TrimSpace(e.EventID)); err != nil {
		return fmt.Errorf("%w: invalid event_id: %v", ErrInvalidLinkVisited, err)
	}
	if strings.TrimSpace(e.RequestID) == "" {
		return fmt.Errorf("%w: request_id must not be empty", ErrInvalidLinkVisited)
	}
	if _, err := uuid.Parse(strings.TrimSpace(e.LinkID)); err != nil {
		return fmt.Errorf("%w: invalid link_id: %v", ErrInvalidLinkVisited, err)
	}
	if strings.TrimSpace(e.ShortCode) == "" {
		return fmt.Errorf("%w: short_code must not be empty", ErrInvalidLinkVisited)
	}
	if e.OccurredAt.IsZero() {
		return fmt.Errorf("%w: occurred_at must not be zero", ErrInvalidLinkVisited)
	}
	if strings.TrimSpace(e.IP) == "" {
		return fmt.Errorf("%w: ip must not be empty", ErrInvalidLinkVisited)
	}
	return nil
}

func MarshalLinkVisited(event *LinkVisited) ([]byte, error) {
	if err := event.Validate(); err != nil {
		return nil, err
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return nil, fmt.Errorf("marshal link.visited: %w", err)
	}
	return payload, nil
}

func UnmarshalLinkVisited(payload []byte) (*LinkVisited, error) {
	var event LinkVisited
	if err := json.Unmarshal(payload, &event); err != nil {
		return nil, fmt.Errorf("unmarshal link.visited: %w", err)
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return &event, nil
}
