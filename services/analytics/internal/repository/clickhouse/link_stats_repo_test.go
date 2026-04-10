package clickhouse

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/google/uuid"
)

type stubRow struct {
	scanFn func(dest ...any) error
}

func (s *stubRow) Err() error                  { return nil }
func (s *stubRow) Scan(dest ...any) error      { return s.scanFn(dest...) }
func (s *stubRow) ScanStruct(dest any) error   { return errors.New("not implemented") }

type stubRows struct {
	items   [][]any
	idx     int
	scanErr error
	rowsErr error
}

func (s *stubRows) Next() bool { return s.idx < len(s.items) }
func (s *stubRows) Scan(dest ...any) error {
	if s.scanErr != nil {
		return s.scanErr
	}
	row := s.items[s.idx]
	s.idx++
	for i := range dest {
		switch d := dest[i].(type) {
		case *time.Time:
			*d = row[i].(time.Time)
		case *uint64:
			*d = row[i].(uint64)
		case *string:
			*d = row[i].(string)
		default:
			return errors.New("unsupported dest type")
		}
	}
	return nil
}
func (s *stubRows) ScanStruct(dest any) error       { return errors.New("not implemented") }
func (s *stubRows) ColumnTypes() []driver.ColumnType { return nil }
func (s *stubRows) Totals(dest ...any) error         { return nil }
func (s *stubRows) Columns() []string                { return nil }
func (s *stubRows) Close() error                     { return nil }
func (s *stubRows) Err() error                       { return s.rowsErr }
func (s *stubRows) HasData() bool                    { return len(s.items) > 0 }

type stubStatsConn struct {
	queryRowFn func(ctx context.Context, query string, args ...any) driver.Row
	queryFn    func(ctx context.Context, query string, args ...any) (driver.Rows, error)
}

func (s *stubStatsConn) QueryRow(ctx context.Context, query string, args ...any) driver.Row {
	return s.queryRowFn(ctx, query, args...)
}

func (s *stubStatsConn) Query(ctx context.Context, query string, args ...any) (driver.Rows, error) {
	return s.queryFn(ctx, query, args...)
}

func TestLinkStatsRepo_GetByLinkID_Success(t *testing.T) {
	linkID := uuid.New()
	now := time.Date(2026, 4, 10, 12, 0, 0, 0, time.UTC)
	lastVisited := now.Add(-time.Hour)
	hourBucket := now.Add(-2 * time.Hour)
	conn := &stubStatsConn{
		queryRowFn: func(_ context.Context, query string, args ...any) driver.Row {
			if len(args) != 1 || args[0] != linkID {
				t.Fatalf("unexpected totals args: %#v", args)
			}
			return &stubRow{
				scanFn: func(dest ...any) error {
					*dest[0].(*uint64) = 7
					*dest[1].(**time.Time) = &lastVisited
					return nil
				},
			}
		},
		queryFn: func(_ context.Context, query string, args ...any) (driver.Rows, error) {
			switch {
			case strings.Contains(query, "toStartOfHour"):
				if args[0] != linkID {
					t.Fatalf("unexpected hourly link id: %#v", args[0])
				}
				cutoff, ok := args[1].(time.Time)
				if !ok || !cutoff.Equal(now.Add(-24*time.Hour)) {
					t.Fatalf("unexpected hourly cutoff: %#v", args[1])
				}
				return &stubRows{
					items: [][]any{
						{hourBucket, uint64(3)},
					},
				}, nil
			default:
				if args[0] != linkID || args[1] != topReferrersLimit {
					t.Fatalf("unexpected referrer args: %#v", args)
				}
				return &stubRows{
					items: [][]any{
						{"https://example.com", uint64(4)},
						{"https://google.com", uint64(2)},
					},
				}, nil
			}
		},
	}
	repo := NewLinkStatsRepo(conn)

	stats, err := repo.GetByLinkID(context.Background(), linkID, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.TotalClicks != 7 {
		t.Fatalf("total clicks = %d, want 7", stats.TotalClicks)
	}
	if stats.LastVisitedAt == nil || !stats.LastVisitedAt.Equal(lastVisited) {
		t.Fatalf("last visited = %v, want %v", stats.LastVisitedAt, lastVisited)
	}
	if len(stats.Hourly) != 1 || !stats.Hourly[0].BucketStart.Equal(hourBucket) || stats.Hourly[0].Clicks != 3 {
		t.Fatalf("unexpected hourly stats: %+v", stats.Hourly)
	}
	if len(stats.TopReferrers) != 2 || stats.TopReferrers[0].Referer != "https://example.com" {
		t.Fatalf("unexpected top referrers: %+v", stats.TopReferrers)
	}
}

func TestLinkStatsRepo_GetByLinkID_WrapsTotalsError(t *testing.T) {
	linkID := uuid.New()
	repo := NewLinkStatsRepo(&stubStatsConn{
		queryRowFn: func(_ context.Context, _ string, _ ...any) driver.Row {
			return &stubRow{
				scanFn: func(dest ...any) error {
					return errors.New("totals boom")
				},
			}
		},
		queryFn: func(_ context.Context, _ string, _ ...any) (driver.Rows, error) {
			t.Fatal("query must not be called after totals error")
			return nil, nil
		},
	})

	_, err := repo.GetByLinkID(context.Background(), linkID, time.Now().UTC())
	if err == nil || !strings.Contains(err.Error(), "clickhouse stats totals") {
		t.Fatalf("expected wrapped totals error, got %v", err)
	}
}
