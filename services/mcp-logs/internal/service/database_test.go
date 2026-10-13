package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"sandbox_url_shortener/services/mcp-logs/internal/domain"
)

type fakeDatabaseRepository struct {
	page          domain.DatabasePage
	err           error
	called        int
	sql           string
	parameters    []any
	maxRows       int
	queryDeadline bool
}

func (f *fakeDatabaseRepository) Describe(context.Context) ([]domain.DatabaseRelation, error) {
	return []domain.DatabaseRelation{{Schema: "mcp_investigation", Name: "links", Columns: []domain.DatabaseColumn{{Name: "id", Type: "uuid"}}}}, f.err
}
func (f *fakeDatabaseRepository) WithReadOnly(ctx context.Context, timeout, lockTimeout time.Duration, fn func(context.Context, DatabaseReader) error) error {
	f.called++
	if timeout <= 0 || lockTimeout <= 0 {
		return errors.New("missing transaction timeouts")
	}
	if f.err != nil {
		return f.err
	}
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return fn(queryCtx, f)
}
func (f *fakeDatabaseRepository) Query(ctx context.Context, sql string, parameters []any, maxRows int) (domain.DatabasePage, error) {
	f.sql = sql
	f.parameters = parameters
	f.maxRows = maxRows
	_, f.queryDeadline = ctx.Deadline()
	return f.page, f.err
}

func TestDatabaseInvestigationSuccessAndParameterPrecision(t *testing.T) {
	repo := &fakeDatabaseRepository{page: domain.DatabasePage{Columns: []domain.DatabaseColumn{{Name: "id", Type: "int"}}, Rows: [][]any{{int64(9223372036854775807)}, {nil}}}}
	svc := NewDatabaseInvestigation(map[string]DatabaseRepository{"shortener": repo}, DatabaseOptions{})
	result, err := svc.QueryDatabase(context.Background(), DatabaseQuery{Database: "shortener", SQL: "SELECT $1::bigint", Parameters: []DatabaseParameter{{Type: "int", Value: json.Number("9223372036854775807")}}})
	if err != nil {
		t.Fatal(err)
	}
	if repo.parameters[0] != int64(9223372036854775807) {
		t.Fatal("int64 bind precision lost")
	}
	if repo.called != 1 || repo.maxRows != 31 || !strings.Contains(repo.sql, "LIMIT 31") {
		t.Fatalf("read-only bounds not applied: %+v", repo)
	}
	if !repo.queryDeadline {
		t.Fatal("transaction deadline was not propagated into query")
	}
	if result.Truncation.Total == nil || *result.Truncation.Total != 2 || result.Truncation.Capped || result.Rows[1][0] != nil {
		t.Fatalf("bad complete result: %+v", result)
	}
}

func TestDatabaseInvestigationDiscoveryAndOptionalConfiguration(t *testing.T) {
	svc := NewDatabaseInvestigation(nil, DatabaseOptions{})
	list, err := svc.ListDatabases(context.Background())
	if err != nil || len(list.Databases) != 0 {
		t.Fatalf("optional database startup failed: %v", err)
	}
	if _, err := svc.DescribeDatabase(context.Background(), "credential-secret"); err == nil || strings.Contains(err.Error(), "credential-secret") {
		t.Fatalf("unknown alias leaked input: %v", err)
	}
	svc = NewDatabaseInvestigation(map[string]DatabaseRepository{"shortener": &fakeDatabaseRepository{}}, DatabaseOptions{})
	description, err := svc.DescribeDatabase(context.Background(), "shortener")
	if err != nil || len(description.Relations) != 1 {
		t.Fatalf("schema discovery failed: %v", err)
	}
}

func TestDatabaseInvestigationRejectsPolicyBeforeRepository(t *testing.T) {
	repo := &fakeDatabaseRepository{}
	svc := NewDatabaseInvestigation(map[string]DatabaseRepository{"shortener": repo}, DatabaseOptions{})
	_, err := svc.QueryDatabase(context.Background(), DatabaseQuery{Database: "shortener", SQL: "SELECT pg_sleep(1)"})
	if err == nil || repo.called != 0 {
		t.Fatal("unsafe SQL reached repository")
	}
}

func TestDatabaseInvestigationRowTruncationDistinguishesExactLimit(t *testing.T) {
	for _, count := range []int{0, 2, 3} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			rows := make([][]any, count)
			for i := range rows {
				rows[i] = []any{int64(i)}
			}
			repo := &fakeDatabaseRepository{page: domain.DatabasePage{Columns: []domain.DatabaseColumn{{Name: "n", Type: "int"}}, Rows: rows}}
			svc := NewDatabaseInvestigation(map[string]DatabaseRepository{"shortener": repo}, DatabaseOptions{MaxRows: 2})
			result, err := svc.QueryDatabase(context.Background(), DatabaseQuery{Database: "shortener", SQL: "SELECT 1", Limit: 100})
			if err != nil {
				t.Fatal(err)
			}
			if count <= 2 {
				if result.Truncation.Total == nil || result.Truncation.Rows {
					t.Fatal("exact limit mislabeled truncated")
				}
			} else {
				if result.Truncation.Total != nil || !result.Truncation.Rows || result.Truncation.TotalAtLeast != 3 {
					t.Fatal("bounded sample invented exact count")
				}
			}
		})
	}
}

func TestDatabaseInvestigationFieldRedactionUTF8AndWireBudget(t *testing.T) {
	rows := make([][]any, 10)
	for i := range rows {
		rows[i] = []any{strings.Repeat("тест ", 200), "password='credential secret'"}
	}
	repo := &fakeDatabaseRepository{page: domain.DatabasePage{Columns: []domain.DatabaseColumn{{Name: "value", Type: "string"}, {Name: "value", Type: "string"}}, Rows: rows}}
	svc := NewDatabaseInvestigation(map[string]DatabaseRepository{"shortener": repo}, DatabaseOptions{FieldBytes: 40, MaxBytes: 2000})
	result, err := svc.QueryDatabase(context.Background(), DatabaseQuery{Database: "shortener", SQL: "SELECT 1"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Truncation.Bytes || result.Truncation.Fields != len(result.Rows) {
		t.Fatalf("bad truncation: %+v", result.Truncation)
	}
	for _, row := range result.Rows {
		if len(row[0].(string)) > 40+len("...(truncated)") || !utf8.ValidString(row[0].(string)) {
			t.Fatal("field cap/UTF8 violated")
		}
		if strings.Contains(row[1].(string), "credential") {
			t.Fatal("secret exposed")
		}
	}
	bytes, err := databaseWireBytes(result)
	if err != nil || bytes > 2000 {
		t.Fatalf("wire budget exceeded: %d %v", bytes, err)
	}
	if len(result.Columns) != 2 || result.Columns[0].Name != result.Columns[1].Name {
		t.Fatal("ordered duplicate aliases lost")
	}
}

func TestDatabaseInvestigationFailurePropagatesSafely(t *testing.T) {
	repo := &fakeDatabaseRepository{err: errors.New("PostgreSQL database unavailable")}
	svc := NewDatabaseInvestigation(map[string]DatabaseRepository{"shortener": repo}, DatabaseOptions{})
	if _, err := svc.QueryDatabase(context.Background(), DatabaseQuery{Database: "shortener", SQL: "SELECT 1"}); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("missing repository failure: %v", err)
	}
}

func TestDatabaseInvestigationScanBudgetMeansUnknownTotal(t *testing.T) {
	repo := &fakeDatabaseRepository{page: domain.DatabasePage{Columns: []domain.DatabaseColumn{{Name: "n", Type: "int"}}, Rows: [][]any{{int64(1)}}, BytesTruncated: true, ObservedRows: 2}}
	svc := NewDatabaseInvestigation(map[string]DatabaseRepository{"shortener": repo}, DatabaseOptions{})
	result, err := svc.QueryDatabase(context.Background(), DatabaseQuery{Database: "shortener", SQL: "SELECT 1"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Truncation.Total != nil || result.Truncation.TotalAtLeast != 2 || !result.Truncation.Bytes || !result.Truncation.Capped {
		t.Fatalf("scan cutoff invented exact total: %+v", result.Truncation)
	}
}

func TestDatabaseParameterValidation(t *testing.T) {
	valid := []DatabaseParameter{{Type: "string", Value: "abc"}, {Type: "bool", Value: true}, {Type: "int", Value: float64(5)}, {Type: "float", Value: json.Number("1.5")}, {Type: "null"}}
	if _, err := databaseParameters(valid); err != nil {
		t.Fatal(err)
	}
	for _, parameter := range []DatabaseParameter{{Type: "string", Value: 5}, {Type: "bool", Value: "true"}, {Type: "int", Value: 1.5}, {Type: "null", Value: "x"}, {Type: "evil", Value: "x"}} {
		if _, err := databaseParameters([]DatabaseParameter{parameter}); err == nil {
			t.Fatalf("bad parameter accepted: %+v", parameter)
		}
	}
}
