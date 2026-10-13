package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"sandbox_url_shortener/services/mcp-logs/internal/domain"
	"sandbox_url_shortener/services/mcp-logs/internal/service"
)

type databaseTransportStub struct {
	bound []any
	calls int
}

func (r *databaseTransportStub) Describe(context.Context) ([]domain.DatabaseRelation, error) {
	return []domain.DatabaseRelation{{Schema: "mcp_investigation", Name: "links"}}, nil
}

func (r *databaseTransportStub) WithReadOnly(ctx context.Context, _, _ time.Duration, fn func(context.Context, domain.DatabaseReader) error) error {
	r.calls++
	return fn(ctx, r)
}

func (r *databaseTransportStub) Query(_ context.Context, _ string, bound []any, _ int) (domain.DatabasePage, error) {
	r.bound = bound
	return domain.DatabasePage{Columns: []domain.DatabaseColumn{{Name: "value", Type: "int"}}, Rows: [][]any{{bound[0]}}}, nil
}

func TestDatabaseToolRegistrationAndIntegerBinding(t *testing.T) {
	repo := &databaseTransportStub{}
	s := newTestServer()
	s.databases = service.NewDatabaseInvestigation(map[string]service.DatabaseRepository{"shortener": repo}, service.DatabaseOptions{})
	if len(s.definitions()) != 9 {
		t.Fatal("database tools missing from shared catalog")
	}
	result, err := s.dispatch(context.Background(), toolQueryDatabase, json.RawMessage(`{"database":"shortener","sql":"SELECT $1::bigint AS value","parameters":[{"type":"int","value":9007199254740993}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if repo.bound[0] != int64(9007199254740993) {
		t.Fatalf("integer parameter lost precision: %v", repo.bound)
	}
	encoded, err := json.Marshal(result)
	if err != nil || !strings.Contains(string(encoded), "9007199254740993") {
		t.Fatalf("unsafe integer result: %s, %v", encoded, err)
	}
}

func TestDatabaseArgumentsFailWithoutEchoingInputOrCallingBackend(t *testing.T) {
	repo := &databaseTransportStub{}
	s := newTestServer()
	s.databases = service.NewDatabaseInvestigation(map[string]service.DatabaseRepository{"shortener": repo}, service.DatabaseOptions{})
	for _, raw := range []string{
		`{"database":"shortener","sql":"SELECT 1","secret-password":"leak"}`,
		`{"database":"shortener","sql":123,"parameters":[{"type":"string","value":"secret-password"}]}`,
		`{"database":"shortener","sql":"SELECT 1"} {"secret-password":true}`,
	} {
		_, err := s.dispatch(context.Background(), toolQueryDatabase, json.RawMessage(raw))
		if err == nil || strings.Contains(err.Error(), "secret-password") {
			t.Fatalf("unsafe argument error: %v", err)
		}
	}
	if repo.calls != 0 {
		t.Fatal("invalid input reached backend")
	}
}
