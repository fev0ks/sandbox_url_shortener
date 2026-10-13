package postgresql

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"sandbox_url_shortener/services/mcp-logs/internal/domain"
)

func TestSafeFailureDiscardsUpstreamSecrets(t *testing.T) {
	for _, test := range []struct {
		err  error
		want string
	}{
		{errors.New("connection postgres://user:credential-secret@db"), "database unavailable"},
		{&pgconn.PgError{Code: "42501", Message: "password=credential-secret", Detail: "sensitive SQL and parameters"}, "SQLSTATE 42501"},
		{&pgconn.PgError{Code: "25006", Message: "sensitive write"}, "read-only violation"},
		{&pgconn.PgError{Code: "57014", Message: "secret timeout"}, "timeout or cancellation"},
		{&pgconn.PgError{Code: "credential-secret", Message: "secret"}, "query failed"},
		{context.Canceled, "request cancelled"},
		{context.DeadlineExceeded, "timeout"},
	} {
		got := safeFailure(test.err).Error()
		if !strings.Contains(got, test.want) || strings.Contains(got, "credential-secret") || strings.Contains(got, "sensitive") {
			t.Fatalf("unsafe failure: %s", got)
		}
	}
}

func TestNewRepoIsLazyAndInvalidDSNErrorsAreSafe(t *testing.T) {
	repo, err := NewRepo(context.Background(), "postgres://mcp_reader:dev@127.0.0.1:1/unavailable?sslmode=disable")
	if err != nil {
		t.Fatalf("lazy pool required online database: %v", err)
	}
	repo.Close()
	_, err = NewRepo(context.Background(), "postgres://user:credential-secret@%invalid")
	if err == nil || strings.Contains(err.Error(), "credential-secret") {
		t.Fatalf("invalid DSN error leaked input: %v", err)
	}
}

func TestDecodeValuePreservesBigIntAndSafeTypes(t *testing.T) {
	value, err := decodeValue(pgtype.Int8OID, []byte("9223372036854775807"))
	if err != nil || value != int64(9223372036854775807) {
		t.Fatalf("big integer precision lost: %v %v", value, err)
	}
	if _, ok := resultType(pgtype.RecordOID); ok {
		t.Fatal("composite result types must be rejected")
	}
	if value, err := decodeValue(pgtype.BoolOID, []byte("t")); err != nil || value != true {
		t.Fatal("boolean decode failed")
	}
	if value, err := decodeValue(pgtype.Float8OID, []byte("NaN")); err != nil || value != "NaN" {
		t.Fatal("nonfinite number must stay JSON-safe")
	}
}

func TestDatabaseRowBoundsBeforeRetainingValues(t *testing.T) {
	columns := []pgconn.FieldDescription{{Name: "numeric", DataTypeOID: pgtype.NumericOID}, {Name: "secret", DataTypeOID: pgtype.TextOID}}
	large := []byte(strings.Repeat("9", 100000))
	secret := []byte("postgres://user:" + strings.Repeat("credential-secret", 100) + "@host")
	row, size, err := decodeDatabaseRow(columns, [][]byte{large, secret})
	if err != nil || size > 200 || row[0] != withheldLargeValue || row[1] != withheldLargeValue {
		t.Fatalf("oversized values retained: %v %d %v", row, size, err)
	}
	if _, err := databaseResultColumns(make([]pgconn.FieldDescription, 65)); err == nil {
		t.Fatal("column count must be refused before reading rows")
	}
}

// The integration test deliberately bypasses service AST validation to prove
// that reader privileges and READ ONLY enforce their independent boundaries.
func TestRestrictedReaderIntegration(t *testing.T) {
	dsn := os.Getenv("MCP_LOGS_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("set MCP_LOGS_TEST_POSTGRES_DSN to the provisioned mcp_reader DSN")
	}
	repo, err := NewRepo(context.Background(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()
	relations, err := repo.Describe(context.Background())
	if err != nil || len(relations) != 1 || relations[0].Name != approvedRelation {
		t.Fatalf("approved view unavailable: %+v %v", relations, err)
	}
	err = repo.WithReadOnly(context.Background(), time.Second, 100*time.Millisecond, func(queryCtx context.Context, reader domain.DatabaseReader) error {
		deadline, ok := queryCtx.Deadline()
		if !ok || time.Until(deadline) > time.Second {
			t.Fatal("transaction scope must bound background callers")
		}
		page, err := reader.Query(queryCtx, "SELECT $1::uuid AS id, $2::timestamptz AS at, $3::bigint AS n, $4::text AS empty", []any{"00000000-0000-0000-0000-000000000001", "2026-10-07T00:00:00Z", int64(9223372036854775807), nil}, 1)
		if err == nil && (len(page.Rows) != 1 || page.Rows[0][2] != int64(9223372036854775807) || page.Rows[0][3] != nil) {
			t.Fatal("bound values lost type or precision")
		}
		return err
	})
	if err != nil {
		t.Fatalf("useful typed parameters failed: %v", err)
	}
	expressions := make([]string, 64)
	for i := range expressions {
		expressions[i] = "1e100000::numeric"
	}
	err = repo.WithReadOnly(context.Background(), 5*time.Second, 100*time.Millisecond, func(queryCtx context.Context, reader domain.DatabaseReader) error {
		page, err := reader.Query(queryCtx, "SELECT "+strings.Join(expressions, ","), nil, 1)
		if err != nil {
			return err
		}
		if len(page.Rows) != 1 || len(page.Rows[0]) != 64 {
			t.Fatal("large-cell query lost row shape")
		}
		for _, cell := range page.Rows[0] {
			if cell != withheldLargeValue {
				t.Fatal("large numeric cell retained")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("large numeric scan was not bounded: %v", err)
	}
	for _, sql := range []string{
		"UPDATE public.links SET short_code='should-never-write' WHERE false",
		"SELECT password_hash FROM public.password_credentials LIMIT 1",
		"CREATE TEMP TABLE mcp_must_not_create(id int)",
	} {
		err := repo.WithReadOnly(context.Background(), time.Second, 100*time.Millisecond, func(queryCtx context.Context, reader domain.DatabaseReader) error {
			_, err := reader.Query(queryCtx, sql, nil, 1)
			return err
		})
		if err == nil {
			t.Fatalf("database enforcement accepted forbidden query: %s", sql)
		}
		var failure *FailureError
		if !errors.As(err, &failure) || (failure.SQLState != "42501" && failure.SQLState != "25006") {
			t.Fatalf("unexpected enforcement error: %v", err)
		}
	}
	err = repo.WithReadOnly(context.Background(), 100*time.Millisecond, 50*time.Millisecond, func(queryCtx context.Context, reader domain.DatabaseReader) error {
		_, err := reader.Query(queryCtx, "SELECT pg_catalog.pg_sleep(1)", nil, 1)
		return err
	})
	if err == nil {
		t.Fatal("statement timeout did not cancel query")
	}
	err = repo.WithReadOnly(context.Background(), time.Second, 100*time.Millisecond, func(queryCtx context.Context, reader domain.DatabaseReader) error {
		page, err := reader.Query(queryCtx, "SELECT 1::bigint AS value", nil, 1)
		if err == nil && (len(page.Rows) != 1 || page.Rows[0][0] != int64(1)) {
			t.Fatal("pool not reusable after failures")
		}
		return err
	})
	if err != nil {
		t.Fatalf("cleanup failed after cancelled query: %v", err)
	}
}
