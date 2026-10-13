package service

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"sandbox_url_shortener/services/mcp-logs/internal/domain"
)

// maxMessageLen caps a single message. A very long field can otherwise push the
// agent's own instructions out of the context window on its own.
const maxMessageLen = 600

// maxFieldLen caps the short identifier-shaped fields. They are normally a few
// dozen characters, but several of them are attacker-reachable: request_id is
// taken from the inbound X-Request-ID header, so its length is whatever the
// caller decided to send.
const maxFieldLen = 128

// Patterns for values that must never leave the log index. Logs routinely pick
// up credentials from headers and payloads, and everything returned here is
// forwarded to a model provider.
var (
	// The optional scheme group matters: in "Authorization: Bearer <token>" the
	// value is two words, and matching a single one would leave the token exposed.
	secretAssignment = regexp.MustCompile(`(?i)\b(password|passwd|secret|token|api[_-]?key|authorization|bearer|refresh[_-]?token|session)\b\s*[:=]\s*(?:(?:bearer|basic|token)\s+)?(?:'(?:\\.|[^'\\])*(?:'|$)|"(?:\\.|[^"\\])*(?:"|$)|\S+)`)
	// Userinfo can hold a short or percent-encoded password that neither an
	// assignment nor the opaque-token rule recognizes. Drop it as a whole,
	// through the last @ in the authority: URL parsers accept raw @ in passwords.
	urlUserinfo = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)[^/?#\s]+@`)
	bearerToken = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._\-]{8,}`)
	longOpaque  = regexp.MustCompile(`\b[A-Za-z0-9._\-]{40,}\b`)
	jwtLike     = regexp.MustCompile(`\beyJ[A-Za-z0-9._\-]{10,}`)

	// repeatedPlaceholder collapses placeholders left adjacent to each other
	// when two rules matched overlapping parts of the same value.
	repeatedPlaceholder = regexp.MustCompile(`(?:\[redacted\]\s*){2,}`)
)

const redactedPlaceholder = "[redacted]"

// redact removes credential-shaped substrings and caps the length.
//
// It is deliberately aggressive: a false positive costs one unreadable value,
// a false negative ships a live token to a third party.
func redact(s string) string {
	if s == "" {
		return ""
	}
	s = urlUserinfo.ReplaceAllString(s, "${1}"+redactedPlaceholder+"@")
	// Order matters: the assignment rule swallows "Authorization: Bearer xxx"
	// whole, so it runs before the rules that would match only part of it.
	s = secretAssignment.ReplaceAllString(s, "$1="+redactedPlaceholder)
	s = bearerToken.ReplaceAllString(s, "Bearer "+redactedPlaceholder)
	s = jwtLike.ReplaceAllString(s, redactedPlaceholder)
	s = longOpaque.ReplaceAllString(s, redactedPlaceholder)
	s = repeatedPlaceholder.ReplaceAllString(s, redactedPlaceholder+" ")

	return strings.TrimSpace(clip(s, maxMessageLen))
}

// clip caps a string without splitting a multi-byte rune. Cutting mid-rune does
// not fail, it just leaves a replacement character in the middle of a word.
func clip(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "...(truncated)"
}

// sanitizeEntry prepares one record for the model.
//
// Redaction used to apply to the message alone, which missed the field where a
// credential most often appears: a URL path carrying a token in its query
// string. The identifier-shaped fields are length-capped rather than redacted,
// because they are exact values an investigator matches on and rewriting them
// would break the next tool call.
func sanitizeEntry(e *domain.LogEntry) {
	e.Message = redact(e.Message)
	e.Error = redact(e.Error)
	e.Path = redact(e.Path)

	e.Service = clip(e.Service, maxFieldLen)
	e.Level = clip(e.Level, maxFieldLen)
	e.Operation = clip(e.Operation, maxFieldLen)
	e.ErrorKind = clip(e.ErrorKind, maxFieldLen)
	e.RequestID = clip(e.RequestID, maxFieldLen)
	e.TraceID = clip(e.TraceID, maxFieldLen)
	e.Method = clip(e.Method, maxFieldLen)

	// Service-specific fields are discovered, so their content is not known in
	// advance. They get the same treatment as a message rather than the milder
	// treatment given to fields whose shape is part of the contract.
	for name, value := range e.Fields {
		e.Fields[name] = redact(value)
	}
}

// Patterns for the variable parts of an otherwise identical message.
var (
	uuidPattern  = regexp.MustCompile(`(?i)\b[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\b`)
	hexPattern   = regexp.MustCompile(`(?i)\b[0-9a-f]{16,}\b`)
	numberInWord = regexp.MustCompile(`\b\w*\d[\w\d]*\b`)
	durationLike = regexp.MustCompile(`\b\d+(\.\d+)?(ns|us|ms|s|m|h)\b`)
	spaces       = regexp.MustCompile(`\s+`)
)

// normalizeMessage collapses the variable parts of a message so that repeated
// failures group together.
//
// Grouping on the exact text almost never works: real messages embed ids,
// codes and durations, so every occurrence looks unique and the grouping
// silently does nothing.
func normalizeMessage(s string) string {
	if s == "" {
		return ""
	}
	s = uuidPattern.ReplaceAllString(s, "<id>")
	s = hexPattern.ReplaceAllString(s, "<hex>")
	s = durationLike.ReplaceAllString(s, "<dur>")
	s = numberInWord.ReplaceAllString(s, "<val>")
	s = spaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
