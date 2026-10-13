package service

import (
	"strings"
	"testing"
)

func TestDatabaseSQLAllowsUsefulReads(t *testing.T) {
	cases := []struct {
		sql        string
		parameters int
	}{
		{"SELECT id, short_code FROM mcp_investigation.links WHERE short_code=$1", 1},
		{"SELECT count(*) FROM mcp_investigation.links", 0},
		{"SELECT pg_catalog.lower(short_code), count(*) FROM mcp_investigation.links GROUP BY short_code ORDER BY count(*) DESC", 0},
		{"WITH recent AS (SELECT id, short_code FROM mcp_investigation.links WHERE created_at > $1::timestamptz) SELECT recent.id FROM recent", 1},
		{"WITH a AS (SELECT id FROM mcp_investigation.links), b AS (SELECT id FROM a) SELECT b.id FROM b", 0},
		{"SELECT a.id, b.short_code FROM mcp_investigation.links a JOIN mcp_investigation.links b ON a.id=b.id", 0},
		{"SELECT id FROM mcp_investigation.links WHERE id IN (SELECT id FROM mcp_investigation.links WHERE deleted_at IS NULL)", 0},
		{"SELECT CASE WHEN max_clicks > 0 THEN 'limited' ELSE 'unlimited' END, coalesce(expires_at, created_at) FROM mcp_investigation.links", 0},
		{"SELECT id FROM (SELECT id FROM mcp_investigation.links) a ORDER BY id LIMIT 10", 0},
		{"SELECT short_code FROM mcp_investigation.links WHERE short_code LIKE 'abc%'", 0},
		{"SELECT 1", 0},
		{"SELECT $1::bigint", 1},
	}
	for _, test := range cases {
		t.Run(test.sql, func(t *testing.T) {
			canonical, err := validateDatabaseSQL(test.sql, test.parameters)
			if err != nil {
				t.Fatalf("useful SELECT rejected: %v", err)
			}
			if canonical == "" {
				t.Fatal("canonical SQL is empty")
			}
		})
	}
}

func TestDatabaseSQLDeniesUnsafeAndUnsupportedForms(t *testing.T) {
	cases := []string{
		"SELECT id FROM public.links", "SELECT password_hash FROM public.password_credentials",
		"SELECT token_hash FROM public.sessions", "SELECT state_hash FROM public.oauth_states",
		"SELECT id FROM links", "SELECT * FROM mcp_investigation.links", "SELECT l.* FROM mcp_investigation.links l",
		"SELECT 1; SELECT 2", "SELECT 1; /*hidden*/ UPDATE public.links SET title='x'",
		"WITH x AS (DELETE FROM public.links RETURNING id) SELECT id FROM x",
		"WITH RECURSIVE x AS (SELECT 1 UNION ALL SELECT 1 FROM x) SELECT 1 FROM x",
		"WITH links AS (SELECT password_hash FROM public.password_credentials) SELECT password_hash FROM links",
		"WITH x AS (SELECT id FROM x) SELECT id FROM x",
		"SELECT id INTO TEMP output FROM mcp_investigation.links",
		"SELECT id FROM mcp_investigation.links FOR UPDATE", "SELECT id FROM mcp_investigation.links FOR SHARE",
		"COPY (SELECT id FROM mcp_investigation.links) TO STDOUT", "CALL evil()", "CREATE TABLE x (id int)", "SET ROLE shortener",
		"SELECT pg_sleep(1)", "SELECT pg_read_file('/etc/passwd')", "SELECT nextval('secret')",
		"SELECT pg_advisory_lock(1)", "SELECT set_config('role','shortener',true)", "SELECT dblink('secret','SELECT 1')",
		"SELECT public.count(1)", "SELECT lower(secret())", "SELECT string_agg(short_code,'') FROM mcp_investigation.links",
		"SELECT 'x'::public.secret_type", "SELECT 'public.sessions'::regclass", "SELECT '1'::int[]",
		"SELECT 1 OPERATOR(public.+) 1", "SELECT 1 OPERATOR(pg_catalog.+) 1",
		"SELECT short_code COLLATE public.evil FROM mcp_investigation.links",
		"SELECT id FROM mcp_investigation.links ORDER BY id USING OPERATOR(public.<)",
		"SELECT id FROM mcp_investigation.links TABLESAMPLE SYSTEM(1)",
		"SELECT x FROM generate_series(1,100) x", "SELECT ARRAY[1,2]", "SELECT current_user",
	}
	for _, sql := range cases {
		t.Run(sql, func(t *testing.T) {
			if _, err := validateDatabaseSQL(sql, 0); err == nil {
				t.Fatal("unsafe SELECT accepted")
			}
		})
	}
}

func TestDatabaseSQLParameterMismatchAndSafeSyntaxErrors(t *testing.T) {
	for _, test := range []struct {
		sql        string
		parameters int
	}{{"SELECT $1", 0}, {"SELECT $2", 2}, {"SELECT 1", 1}} {
		if _, err := validateDatabaseSQL(test.sql, test.parameters); err == nil {
			t.Fatal("parameter mismatch accepted")
		}
	}
	_, err := validateDatabaseSQL("SELECT password=credential-secret !!!", 0)
	if err == nil || strings.Contains(err.Error(), "credential-secret") {
		t.Fatalf("unsafe syntax error: %v", err)
	}
}

func TestDatabaseSQLQualifiesAuditedFunctionsAndCasts(t *testing.T) {
	canonical, err := validateDatabaseSQL("SELECT lower(short_code), $1::text FROM mcp_investigation.links", 1)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(canonical, "pg_catalog.lower") || !strings.Contains(canonical, "pg_catalog.text") {
		t.Fatalf("builtins not qualified: %s", canonical)
	}
}
