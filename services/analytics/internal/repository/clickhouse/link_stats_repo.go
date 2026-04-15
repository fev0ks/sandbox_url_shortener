package clickhouse

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	"sandbox_url_shortener/internal/platform/metrics"
	"sandbox_url_shortener/services/analytics/internal/domain"
)

const topReferrersLimit = 5

type statsConn interface {
	QueryRow(ctx context.Context, query string, args ...any) driver.Row
	Query(ctx context.Context, query string, args ...any) (driver.Rows, error)
}

type LinkStatsRepo struct {
	conn statsConn
}

func NewLinkStatsRepo(conn statsConn) *LinkStatsRepo {
	return &LinkStatsRepo{conn: conn}
}

func (r *LinkStatsRepo) GetByLinkID(ctx context.Context, linkID uuid.UUID, now time.Time) (*domain.LinkStats, error) {
	stats := &domain.LinkStats{}

	if err := r.loadTotals(ctx, stats, linkID); err != nil {
		return nil, err
	}
	if err := r.loadHourly(ctx, stats, linkID, now); err != nil {
		return nil, err
	}
	if err := r.loadTopReferrers(ctx, stats, linkID); err != nil {
		return nil, err
	}

	return stats, nil
}

func (r *LinkStatsRepo) loadTotals(ctx context.Context, stats *domain.LinkStats, linkID uuid.UUID) error {
	start := time.Now()
	ctx, span := otel.Tracer("analytics.clickhouse").Start(ctx, "analytics.clickhouse.stats_totals")
	defer span.End()
	span.SetAttributes(attribute.String("link.id", linkID.String()))

	const q = `
		SELECT
			uniqExact(event_id) AS total_clicks,
			maxOrNull(occurred_at) AS last_visited_at
		FROM click_events_raw
		WHERE link_id = ?
	`

	if err := r.conn.QueryRow(ctx, q, linkID).Scan(&stats.TotalClicks, &stats.LastVisitedAt); err != nil {
		metrics.ObserveClickHouseOperation("analytics", "stats_totals", "failure", time.Since(start))
		span.RecordError(err)
		span.SetStatus(codes.Error, "query_failed")
		return fmt.Errorf("clickhouse stats totals: %w", err)
	}

	metrics.ObserveClickHouseOperation("analytics", "stats_totals", "success", time.Since(start))
	span.SetStatus(codes.Ok, "")
	return nil
}

func (r *LinkStatsRepo) loadHourly(ctx context.Context, stats *domain.LinkStats, linkID uuid.UUID, now time.Time) error {
	start := time.Now()
	ctx, span := otel.Tracer("analytics.clickhouse").Start(ctx, "analytics.clickhouse.stats_hourly")
	defer span.End()
	span.SetAttributes(attribute.String("link.id", linkID.String()))

	const q = `
		SELECT
			toStartOfHour(occurred_at) AS bucket_start,
			uniqExact(event_id) AS clicks
		FROM click_events_raw
		WHERE link_id = ?
		  AND occurred_at >= ?
		GROUP BY bucket_start
		ORDER BY bucket_start ASC
	`

	rows, err := r.conn.Query(ctx, q, linkID, now.UTC().Add(-24*time.Hour))
	if err != nil {
		metrics.ObserveClickHouseOperation("analytics", "stats_hourly", "failure", time.Since(start))
		span.RecordError(err)
		span.SetStatus(codes.Error, "query_failed")
		return fmt.Errorf("clickhouse stats hourly query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var bucket domain.LinkStatsBucket
		if err := rows.Scan(&bucket.BucketStart, &bucket.Clicks); err != nil {
			metrics.ObserveClickHouseOperation("analytics", "stats_hourly", "failure", time.Since(start))
			span.RecordError(err)
			span.SetStatus(codes.Error, "scan_failed")
			return fmt.Errorf("clickhouse stats hourly scan: %w", err)
		}
		stats.Hourly = append(stats.Hourly, bucket)
	}
	if err := rows.Err(); err != nil {
		metrics.ObserveClickHouseOperation("analytics", "stats_hourly", "failure", time.Since(start))
		span.RecordError(err)
		span.SetStatus(codes.Error, "rows_failed")
		return fmt.Errorf("clickhouse stats hourly rows: %w", err)
	}

	metrics.ObserveClickHouseOperation("analytics", "stats_hourly", "success", time.Since(start))
	span.SetStatus(codes.Ok, "")
	return nil
}

func (r *LinkStatsRepo) loadTopReferrers(ctx context.Context, stats *domain.LinkStats, linkID uuid.UUID) error {
	start := time.Now()
	ctx, span := otel.Tracer("analytics.clickhouse").Start(ctx, "analytics.clickhouse.stats_top_referrers")
	defer span.End()
	span.SetAttributes(attribute.String("link.id", linkID.String()))

	const q = `
		SELECT
			referer,
			uniqExact(event_id) AS clicks
		FROM click_events_raw
		WHERE link_id = ?
		  AND referer != ''
		GROUP BY referer
		ORDER BY clicks DESC, referer ASC
		LIMIT ?
	`

	rows, err := r.conn.Query(ctx, q, linkID, topReferrersLimit)
	if err != nil {
		metrics.ObserveClickHouseOperation("analytics", "stats_top_referrers", "failure", time.Since(start))
		span.RecordError(err)
		span.SetStatus(codes.Error, "query_failed")
		return fmt.Errorf("clickhouse stats referrers query: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var ref domain.LinkReferrerStat
		if err := rows.Scan(&ref.Referer, &ref.Clicks); err != nil {
			metrics.ObserveClickHouseOperation("analytics", "stats_top_referrers", "failure", time.Since(start))
			span.RecordError(err)
			span.SetStatus(codes.Error, "scan_failed")
			return fmt.Errorf("clickhouse stats referrers scan: %w", err)
		}
		stats.TopReferrers = append(stats.TopReferrers, ref)
	}
	if err := rows.Err(); err != nil {
		metrics.ObserveClickHouseOperation("analytics", "stats_top_referrers", "failure", time.Since(start))
		span.RecordError(err)
		span.SetStatus(codes.Error, "rows_failed")
		return fmt.Errorf("clickhouse stats referrers rows: %w", err)
	}

	metrics.ObserveClickHouseOperation("analytics", "stats_top_referrers", "success", time.Since(start))
	span.SetStatus(codes.Ok, "")
	return nil
}
