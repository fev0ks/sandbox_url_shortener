package service

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRedactRemovesCredentialShapedValues(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		wantOut string
	}{
		{
			name:    "assignment",
			in:      "auth failed password=hunter2 for user bob",
			wantOut: "auth failed password=[redacted] for user bob",
		},
		{
			name:    "bearer header",
			in:      "upstream rejected Authorization: Bearer abcdefghijklmnop",
			wantOut: "upstream rejected Authorization=[redacted]",
		},
		{
			name:    "jwt",
			in:      "token eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9 expired",
			wantOut: "token [redacted] expired",
		},
		{
			name:    "quoted DSN with spaces",
			in:      `failed to connect host=postgres password='small secret' dbname=shortener`,
			wantOut: `failed to connect host=postgres password=[redacted] dbname=shortener`,
		},
		{
			name:    "escaped single quote in DSN",
			in:      `password='small\' secret' host=postgres`,
			wantOut: `password=[redacted] host=postgres`,
		},
		{
			name:    "escaped double quote in DSN",
			in:      `password="small\" secret" host=postgres`,
			wantOut: `password=[redacted] host=postgres`,
		},
		{
			name:    "unterminated quoted DSN",
			in:      `password='small secret`,
			wantOut: `password=[redacted]`,
		},
		{
			name:    "URL userinfo",
			in:      `failed to connect postgres://shortener:tiny@postgres:5432/shortener: no such host`,
			wantOut: `failed to connect postgres://[redacted]@postgres:5432/shortener: no such host`,
		},
		{
			name:    "percent encoded URL password",
			in:      `postgresql://user:pa%40ss%20word@postgres:5432/db?sslmode=disable`,
			wantOut: `postgresql://[redacted]@postgres:5432/db?sslmode=disable`,
		},
		{
			name:    "unescaped at sign in URL password",
			in:      `postgres://user:tiny@secret@postgres:5432/db: connection refused`,
			wantOut: `postgres://[redacted]@postgres:5432/db: connection refused`,
		},
		{
			name:    "userinfo stops before path",
			in:      `postgres://user:tiny@secret@postgres:5432/db@tenant`,
			wantOut: `postgres://[redacted]@postgres:5432/db@tenant`,
		},
		{
			name:    "userinfo stops before query",
			in:      `postgres://user:tiny@postgres?tenant=a@b`,
			wantOut: `postgres://[redacted]@postgres?tenant=a@b`,
		},
		{
			name:    "userinfo stops before fragment",
			in:      `postgres://user:tiny@postgres#tenant=a@b`,
			wantOut: `postgres://[redacted]@postgres#tenant=a@b`,
		},
		{
			name:    "userinfo stops at whitespace",
			in:      `postgres://user:tiny@postgres:5432/db contact admin@example.com`,
			wantOut: `postgres://[redacted]@postgres:5432/db contact admin@example.com`,
		},
		{
			name:    "database DNS diagnostic",
			in:      `failed to connect to ` + "`user=shortener database=shortener`" + `: lookup postgres on 127.0.0.11:53: no such host`,
			wantOut: `failed to connect to ` + "`user=shortener database=shortener`" + `: lookup postgres on 127.0.0.11:53: no such host`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := redact(tc.in); got != tc.wantOut {
				t.Fatalf("redact() = %q, want %q", got, tc.wantOut)
			}
		})
	}
}

func TestRedactCapsUnicodeAfterRemovingSecrets(t *testing.T) {
	// Unicode text does not resemble an opaque token; this reaches the length
	// limit and forces a cut inside a multi-byte rune without the UTF-8 guard.
	got := redact("cause: " + strings.Repeat("я ", 201) + ` password='hidden secret'`)
	if !utf8.ValidString(got) || len(got) > maxMessageLen+len("...(truncated)") {
		t.Fatalf("redacted diagnostic must be bounded, valid UTF-8: %q", got)
	}
	if !strings.HasSuffix(got, "...(truncated)") || strings.Contains(got, "hidden") {
		t.Fatalf("expected an explicitly truncated diagnostic without secrets: %q", got)
	}
}

func TestRedactCapsLength(t *testing.T) {
	long := make([]byte, maxMessageLen*2)
	for i := range long {
		long[i] = 'x'
	}

	got := redact(string(long))
	if len(got) >= len(long) {
		t.Fatalf("redact() did not cap length: got %d bytes", len(got))
	}
}

func TestNormalizeMessageGroupsVaryingIdentifiers(t *testing.T) {
	first := normalizeMessage("short_code=abc123 not found")
	second := normalizeMessage("short_code=xy9z01 not found")

	if first != second {
		t.Fatalf("messages differing only by identifier must normalize equally: %q != %q", first, second)
	}
}

func TestNormalizeMessageCollapsesUUIDAndDuration(t *testing.T) {
	got := normalizeMessage("request 4f662fd9-f683-5424-60ed-28cb0c6e91aa timed out after 3021ms")
	want := "request <id> timed out after <dur>"

	if got != want {
		t.Fatalf("normalizeMessage() = %q, want %q", got, want)
	}
}

func TestNormalizeMessageKeepsDistinctFailuresApart(t *testing.T) {
	if normalizeMessage("link not found") == normalizeMessage("database unavailable") {
		t.Fatal("unrelated messages must not normalize to the same pattern")
	}
}
