package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"sandbox_url_shortener/services/mcp-logs/internal/domain"
)

// DatabaseReader executes a validated query inside an explicit read-only transaction.
type DatabaseReader = domain.DatabaseReader

// DatabaseRepository owns persistence mechanics, not query authorization.
type DatabaseRepository interface {
	Describe(ctx context.Context) ([]domain.DatabaseRelation, error)
	WithReadOnly(ctx context.Context, timeout, lockTimeout time.Duration, fn func(context.Context, DatabaseReader) error) error
}

// DatabaseOptions sets operator-owned ceilings which callers cannot widen.
type DatabaseOptions struct {
	MaxRows          int
	MaxBytes         int
	FieldBytes       int
	StatementTimeout time.Duration
	LockTimeout      time.Duration
}

// MaxDatabaseRows is the absolute model-facing row ceiling.
const MaxDatabaseRows = 100

const (
	defaultDatabaseRows   = 30
	maxDatabaseBytes      = 32 << 10
	maxDatabaseSQLBytes   = 16 << 10
	maxDatabaseParameters = 32
)

// DatabaseInvestigation implements approved, bounded database investigation reads.
type DatabaseInvestigation struct {
	repos map[string]DatabaseRepository
	opts  DatabaseOptions
}

// NewDatabaseInvestigation constructs an independent, optionally configured use case.
func NewDatabaseInvestigation(repos map[string]DatabaseRepository, options DatabaseOptions) *DatabaseInvestigation {
	copyRepos := make(map[string]DatabaseRepository, len(repos))
	for alias, repo := range repos {
		copyRepos[alias] = repo
	}
	if options.MaxRows <= 0 || options.MaxRows > MaxDatabaseRows {
		options.MaxRows = MaxDatabaseRows
	}
	if options.MaxBytes <= 0 || options.MaxBytes > maxDatabaseBytes {
		options.MaxBytes = maxDatabaseBytes
	}
	if options.FieldBytes <= 0 || options.FieldBytes > maxMessageLen {
		options.FieldBytes = maxMessageLen
	}
	if options.StatementTimeout <= 0 {
		options.StatementTimeout = 5 * time.Second
	}
	if options.LockTimeout <= 0 {
		options.LockTimeout = 500 * time.Millisecond
	}
	return &DatabaseInvestigation{repos: copyRepos, opts: options}
}

// ListDatabases lists configured aliases without contacting unavailable databases.
func (s *DatabaseInvestigation) ListDatabases(_ context.Context) (*domain.DatabaseList, error) {
	aliases := make([]string, 0, len(s.repos))
	for alias := range s.repos {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	return &domain.DatabaseList{Databases: aliases, NextStep: "Call describe_database with one database alias before constructing a SELECT."}, nil
}

// DescribeDatabase lists only approved relations visible to the reader role.
func (s *DatabaseInvestigation) DescribeDatabase(ctx context.Context, alias string) (*domain.DatabaseDescription, error) {
	repo, err := s.repository(alias)
	if err != nil {
		return nil, err
	}
	relations, err := repo.Describe(ctx)
	if err != nil {
		return nil, fmt.Errorf("describe database: %w", err)
	}
	return &domain.DatabaseDescription{Database: alias, Relations: relations, NextStep: "Use explicit columns and bound parameters in query_database. Only mcp_investigation.links is approved."}, nil
}

// DatabaseParameter supplies an explicit JSON-safe scalar bind value.
type DatabaseParameter struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// DatabaseQuery asks for one approved read query over a configured alias.
type DatabaseQuery struct {
	Database   string              `json:"database"`
	SQL        string              `json:"sql"`
	Parameters []DatabaseParameter `json:"parameters"`
	Limit      int                 `json:"limit"`
}

// QueryDatabase validates SQL policy, owns transaction scope and bounds the response.
func (s *DatabaseInvestigation) QueryDatabase(ctx context.Context, q DatabaseQuery) (*domain.DatabaseResult, error) {
	repo, err := s.repository(q.Database)
	if err != nil {
		return nil, err
	}
	parameters, err := databaseParameters(q.Parameters)
	if err != nil {
		return nil, err
	}
	canonical, err := validateDatabaseSQL(q.SQL, len(parameters))
	if err != nil {
		return nil, err
	}
	limit := q.Limit
	if limit <= 0 {
		limit = defaultDatabaseRows
	}
	if limit > s.opts.MaxRows {
		limit = s.opts.MaxRows
	}
	var page domain.DatabasePage
	err = repo.WithReadOnly(ctx, s.opts.StatementTimeout, s.opts.LockTimeout, func(queryCtx context.Context, reader DatabaseReader) error {
		var queryErr error
		// The wrapper is built from a validated AST and a server-owned integer.
		wrapped := fmt.Sprintf("SELECT * FROM (%s) AS mcp_result LIMIT %d", canonical, limit+1)
		page, queryErr = reader.Query(queryCtx, wrapped, parameters, limit+1)
		return queryErr
	})
	if err != nil {
		return nil, fmt.Errorf("query database: %w", err)
	}
	return s.boundedDatabaseResult(q.Database, page, limit)
}

func (s *DatabaseInvestigation) repository(alias string) (DatabaseRepository, error) {
	if repo, ok := s.repos[alias]; ok && repo != nil {
		return repo, nil
	}
	return nil, fmt.Errorf("unknown or unconfigured database alias; call list_databases")
}

func databaseParameters(parameters []DatabaseParameter) ([]any, error) {
	if len(parameters) > maxDatabaseParameters {
		return nil, fmt.Errorf("too many parameters; maximum is %d", maxDatabaseParameters)
	}
	values := make([]any, len(parameters))
	for i, parameter := range parameters {
		value, err := databaseParameter(parameter, i+1)
		if err != nil {
			return nil, err
		}
		values[i] = value
	}
	return values, nil
}

func databaseParameter(parameter DatabaseParameter, index int) (any, error) {
	switch parameter.Type {
	case "null":
		if parameter.Value != nil {
			return nil, fmt.Errorf("parameter %d must be null", index)
		}
		return nil, nil
	case "string":
		value, ok := parameter.Value.(string)
		if !ok || len(value) > maxDatabaseSQLBytes {
			return nil, fmt.Errorf("parameter %d must be a bounded string", index)
		}
		return value, nil
	case "bool":
		value, ok := parameter.Value.(bool)
		if !ok {
			return nil, fmt.Errorf("parameter %d must be a boolean", index)
		}
		return value, nil
	case "int", "float":
		return numericDatabaseParameter(parameter, index)
	default:
		return nil, fmt.Errorf("parameter %d has unsupported type; use string, int, float, bool or null", index)
	}
}

func numericDatabaseParameter(parameter DatabaseParameter, index int) (any, error) {
	number, ok := databaseNumberString(parameter.Value)
	if !ok {
		return nil, fmt.Errorf("parameter %d must be numeric", index)
	}
	if parameter.Type == "int" {
		value, err := strconv.ParseInt(number, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parameter %d must be an int64", index)
		}
		return value, nil
	}
	value, err := strconv.ParseFloat(number, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, fmt.Errorf("parameter %d must be finite", index)
	}
	return value, nil
}

func databaseNumberString(value any) (string, bool) {
	switch number := value.(type) {
	case json.Number:
		return number.String(), true
	case float64:
		return strconv.FormatFloat(number, 'g', -1, 64), true
	case int:
		return strconv.Itoa(number), true
	case int64:
		return strconv.FormatInt(number, 10), true
	default:
		return "", false
	}
}

func (s *DatabaseInvestigation) boundedDatabaseResult(alias string, page domain.DatabasePage, limit int) (*domain.DatabaseResult, error) {
	result := &domain.DatabaseResult{Database: alias, Columns: page.Columns, Rows: make([][]any, 0)}
	if len(result.Columns) > 64 {
		return nil, fmt.Errorf("too many result columns; maximum is 64")
	}
	for i := range result.Columns {
		result.Columns[i].Name = clip(redact(result.Columns[i].Name), maxFieldLen)
		result.Columns[i].Type = clip(result.Columns[i].Type, maxFieldLen)
	}
	rowCount := len(page.Rows)
	if page.BytesTruncated {
		result.Truncation.Bytes = true
		result.Truncation.TotalAtLeast = page.ObservedRows
	}
	if rowCount > limit {
		result.Truncation.Rows = true
		result.Truncation.TotalAtLeast = rowCount
		rowCount = limit
	} else if !page.BytesTruncated {
		total := rowCount
		result.Truncation.Total = &total
	}
	for _, source := range page.Rows[:rowCount] {
		row := make([]any, len(source))
		fields := 0
		for i, value := range source {
			if text, ok := value.(string); ok {
				text = redact(text)
				truncated := strings.HasSuffix(text, "...(truncated)")
				if len(text) > s.opts.FieldBytes {
					text = clip(text, s.opts.FieldBytes)
					truncated = true
				}
				if truncated {
					fields++
				}
				row[i] = text
			} else {
				row[i] = value
			}
		}
		result.Rows = append(result.Rows, row)
		result.Truncation.Returned = len(result.Rows)
		result.Truncation.Fields += fields
		result.Truncation.Capped = result.Truncation.Rows || result.Truncation.Fields > 0
		encodedBytes, err := databaseWireBytes(result)
		if err != nil {
			return nil, fmt.Errorf("unsupported database result encoding")
		}
		// Reserve space for the final truncation flags/metadata and wire text.
		if encodedBytes > s.opts.MaxBytes-256 {
			result.Rows = result.Rows[:len(result.Rows)-1]
			result.Truncation.Fields -= fields
			result.Truncation.Bytes = true
			break
		}
	}
	result.Truncation.Returned = len(result.Rows)
	result.Truncation.Capped = result.Truncation.Rows || result.Truncation.Bytes || result.Truncation.Fields > 0
	if encodedBytes, err := databaseWireBytes(result); err != nil || encodedBytes > s.opts.MaxBytes {
		return nil, fmt.Errorf("database result metadata exceeds response budget; select fewer columns")
	}
	return result, nil
}

func databaseWireBytes(result *domain.DatabaseResult) (int, error) {
	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return 0, err
	}
	text, err := json.Marshal(string(encoded))
	// The MCP content envelope and a JSON-RPC identifier also occupy wire bytes.
	return len(text) + 256, err
}
