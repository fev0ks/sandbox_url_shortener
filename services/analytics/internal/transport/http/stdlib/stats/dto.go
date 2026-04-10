package stats

import (
	"time"

	"sandbox_url_shortener/services/analytics/internal/domain"
)

type Response struct {
	TotalClicks   uint64            `json:"total_clicks"`
	LastVisitedAt *time.Time        `json:"last_visited_at,omitempty"`
	Hourly        []HourlyBucket    `json:"hourly"`
	TopReferrers  []ReferrerSummary `json:"top_referrers"`
}

type HourlyBucket struct {
	BucketStart time.Time `json:"bucket_start"`
	Clicks      uint64    `json:"clicks"`
}

type ReferrerSummary struct {
	Referer string `json:"referer"`
	Clicks  uint64 `json:"clicks"`
}

func toResponse(stats *domain.LinkStats) Response {
	resp := Response{
		TotalClicks:   stats.TotalClicks,
		LastVisitedAt: stats.LastVisitedAt,
		Hourly:        make([]HourlyBucket, 0, len(stats.Hourly)),
		TopReferrers:  make([]ReferrerSummary, 0, len(stats.TopReferrers)),
	}
	for _, bucket := range stats.Hourly {
		resp.Hourly = append(resp.Hourly, HourlyBucket{
			BucketStart: bucket.BucketStart,
			Clicks:      bucket.Clicks,
		})
	}
	for _, ref := range stats.TopReferrers {
		resp.TopReferrers = append(resp.TopReferrers, ReferrerSummary{
			Referer: ref.Referer,
			Clicks:  ref.Clicks,
		})
	}
	return resp
}
