package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type LinkStats struct {
	TotalClicks   uint64
	LastVisitedAt *time.Time
	Hourly        []LinkStatsBucket
	TopReferrers  []LinkReferrerStat
}

type LinkStatsBucket struct {
	BucketStart time.Time
	Clicks      uint64
}

type LinkReferrerStat struct {
	Referer string
	Clicks  uint64
}

type LinkStatsRepository interface {
	GetByLinkID(ctx context.Context, linkID uuid.UUID, now time.Time) (*LinkStats, error)
}
