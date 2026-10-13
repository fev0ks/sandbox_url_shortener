// Package postgresql owns approved PostgreSQL investigation persistence reads.
package postgresql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"sandbox_url_shortener/services/mcp-logs/internal/domain"
)

const (
	approvedSchema     = domain.InvestigationSchema
	approvedRelation   = domain.InvestigationLinksView
	readSearchPath     = "pg_catalog,mcp_investigation,pg_temp"
	maxScanColumns     = 64
	maxScanFieldBytes  = 600
	maxScanBytes       = 32 << 10
	maxScanRows        = 101
	withheldLargeValue = "[value exceeds field limit]...(truncated)"
)

// Repo holds a lazy pool for one operator-configured alias.
type Repo struct{ pool *pgxpool.Pool }

// NewRepo parses server-owned configuration without requiring database availability.
func NewRepo(ctx context.Context, dsn string) (*Repo, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid database connection configuration")
	}
	cfg.MaxConns = 3
	cfg.MinConns = 0
	cfg.ConnConfig.ConnectTimeout = 3 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("invalid database pool configuration")
	}
	return &Repo{pool: pool}, nil
}

// Close releases the pool without affecting unrelated investigation dependencies.
func (r *Repo) Close() { r.pool.Close() }

// FailureError exposes only a safe category and optional SQLSTATE.
type FailureError struct {
	Kind     string
	SQLState string
}

// Error reports no upstream message, connection string, SQL or parameter values.
func (e *FailureError) Error() string {
	if e.SQLState != "" {
		return "PostgreSQL " + e.Kind + " (SQLSTATE " + e.SQLState + ")"
	}
	return "PostgreSQL " + e.Kind
}

func safeFailure(err error) error {
	if err == nil {
		return nil
	}
	var safe *FailureError
	if errors.As(err, &safe) {
		return safe
	}
	if errors.Is(err, context.Canceled) {
		return &FailureError{Kind: "request cancelled"}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &FailureError{Kind: "timeout"}
	}
	var upstream *pgconn.PgError
	if errors.As(err, &upstream) {
		kind := "query failed"
		switch upstream.Code {
		case "42501":
			kind = "permission denied"
		case "25006":
			kind = "read-only violation"
		case "57014", "55P03":
			kind = "timeout or cancellation"
		case "42P01", "42703":
			kind = "relation or column unavailable"
		case "42601":
			kind = "syntax error"
		case "22P02", "22003", "22007", "22008", "42804", "42P18", "42883":
			kind = "parameter or type mismatch"
		}
		return &FailureError{Kind: kind, SQLState: databaseSQLState(upstream.Code)}
	}
	return &FailureError{Kind: "database unavailable"}
}

func databaseSQLState(code string) string {
	if len(code) != 5 {
		return ""
	}
	for _, char := range code {
		if (char < '0' || char > '9') && (char < 'A' || char > 'Z') {
			return ""
		}
	}
	return code
}

// WithReadOnly implements the transaction scope requested by the use-case service.
func (r *Repo) WithReadOnly(ctx context.Context, timeout, lockTimeout time.Duration, fn func(context.Context, domain.DatabaseReader) error) (resultErr error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return safeFailure(err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer stop()
		if rollbackErr := tx.Rollback(cleanup); rollbackErr != nil && !errors.Is(rollbackErr, pgx.ErrTxClosed) && resultErr == nil {
			resultErr = safeFailure(rollbackErr)
		}
	}()
	_, err = tx.Exec(ctx, `SELECT set_config('search_path',$1,true), set_config('statement_timeout',$2,true), set_config('lock_timeout',$3,true)`, readSearchPath, fmt.Sprintf("%dms", max(1, timeout.Milliseconds())), fmt.Sprintf("%dms", max(1, lockTimeout.Milliseconds())))
	if err != nil {
		return safeFailure(err)
	}
	// A misconfigured DSN must not silently upgrade the service to a superuser.
	var privileged bool
	err = tx.QueryRow(ctx, `SELECT r.rolsuper OR r.rolcreatedb OR r.rolcreaterole OR r.rolreplication OR r.rolbypassrls OR r.rolinherit
		OR d.datdba=r.oid
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_auth_members m WHERE m.member=r.oid)
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_namespace n WHERE n.nspowner=r.oid AND n.nspname IN ('public','mcp_investigation'))
		OR EXISTS (SELECT 1 FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE c.relowner=r.oid AND n.nspname IN ('public','mcp_investigation'))
		OR pg_catalog.has_database_privilege(pg_catalog.current_database(),'TEMP')
		OR pg_catalog.has_schema_privilege('public','CREATE')
		OR pg_catalog.has_schema_privilege('mcp_investigation','CREATE')
		FROM pg_catalog.pg_roles r JOIN pg_catalog.pg_database d ON d.datname=pg_catalog.current_database()
		WHERE r.rolname=current_user`).Scan(&privileged)
	if err != nil {
		return safeFailure(err)
	}
	if privileged {
		return &FailureError{Kind: "reader role is privileged; access denied"}
	}
	return safeFailure(fn(ctx, &reader{tx: tx}))
}

// Describe lists only the approved view and columns the current role can read.
func (r *Repo) Describe(ctx context.Context) ([]domain.DatabaseRelation, error) {
	relations := make([]domain.DatabaseRelation, 0)
	err := r.WithReadOnly(ctx, 5*time.Second, 500*time.Millisecond, func(queryCtx context.Context, raw domain.DatabaseReader) error {
		transaction := raw.(*reader)
		rows, err := transaction.tx.Query(queryCtx, `SELECT a.attname, pg_catalog.format_type(a.atttypid,a.atttypmod)
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON c.oid=a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace
		WHERE n.nspname=$1 AND c.relname=$2 AND c.relkind='v'
		AND a.attnum>0 AND NOT a.attisdropped
		AND pg_catalog.has_column_privilege(c.oid,a.attnum,'SELECT')
		ORDER BY a.attnum LIMIT 100`, approvedSchema, approvedRelation)
		if err != nil {
			return safeFailure(err)
		}
		defer rows.Close()
		columns := make([]domain.DatabaseColumn, 0)
		for rows.Next() {
			var column domain.DatabaseColumn
			if err := rows.Scan(&column.Name, &column.Type); err != nil {
				return safeFailure(err)
			}
			columns = append(columns, column)
		}
		if err := rows.Err(); err != nil {
			return safeFailure(err)
		}
		if len(columns) > 0 {
			relations = append(relations, domain.DatabaseRelation{Schema: approvedSchema, Name: approvedRelation, Columns: columns})
		}
		return nil
	})
	return relations, err
}

type reader struct{ tx pgx.Tx }

func (r *reader) Query(ctx context.Context, sql string, parameters []any, maxRows int) (domain.DatabasePage, error) {
	if maxRows <= 0 || maxRows > maxScanRows {
		maxRows = maxScanRows
	}
	args := append([]any{pgx.QueryResultFormats{pgx.TextFormatCode}}, parameters...)
	rows, err := r.tx.Query(ctx, sql, args...)
	if err != nil {
		return domain.DatabasePage{}, safeFailure(err)
	}
	defer rows.Close()
	page := domain.DatabasePage{Rows: make([][]any, 0)}
	fields := rows.FieldDescriptions()
	page.Columns, err = databaseResultColumns(fields)
	if err != nil {
		return domain.DatabasePage{}, err
	}
	retainedBytes := 0
	for rows.Next() {
		if len(page.Rows) >= maxRows {
			break
		}
		page.ObservedRows++
		row, size, decodeErr := decodeDatabaseRow(fields, rows.RawValues())
		if decodeErr != nil {
			return domain.DatabasePage{}, decodeErr
		}
		if retainedBytes+size > maxScanBytes {
			page.BytesTruncated = true
			break
		}
		retainedBytes += size
		page.Rows = append(page.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return domain.DatabasePage{}, safeFailure(err)
	}
	return page, nil
}

func databaseResultColumns(fields []pgconn.FieldDescription) ([]domain.DatabaseColumn, error) {
	if len(fields) > maxScanColumns {
		return nil, &FailureError{Kind: "too many result columns; maximum is 64"}
	}
	columns := make([]domain.DatabaseColumn, 0, len(fields))
	for _, field := range fields {
		name, ok := resultType(field.DataTypeOID)
		if !ok {
			return nil, &FailureError{Kind: "unsupported result type"}
		}
		columns = append(columns, domain.DatabaseColumn{Name: field.Name, Type: name})
	}
	return columns, nil
}

func decodeDatabaseRow(fields []pgconn.FieldDescription, raw [][]byte) ([]any, int, error) {
	if len(fields) != len(raw) {
		return nil, 0, &FailureError{Kind: "invalid result column count"}
	}
	row := make([]any, len(fields))
	for i, field := range fields {
		if i >= len(raw) {
			return nil, 0, &FailureError{Kind: "invalid result column count"}
		}
		value := raw[i]
		if value == nil {
			continue
		}
		// Withhold oversized cells before any string allocation or numeric parse.
		// A partial credential-bearing URL or assignment must never become output.
		if len(value) > maxScanFieldBytes {
			row[i] = withheldLargeValue
			continue
		}
		decoded, err := decodeValue(field.DataTypeOID, value)
		if err != nil {
			return nil, 0, err
		}
		row[i] = decoded
	}
	encoded, err := json.Marshal(row)
	if err != nil {
		return nil, 0, &FailureError{Kind: "unsupported result encoding"}
	}
	return row, len(encoded), nil
}

func resultType(oid uint32) (string, bool) {
	switch oid {
	case pgtype.BoolOID:
		return "bool", true
	case pgtype.Int2OID, pgtype.Int4OID, pgtype.Int8OID:
		return "int", true
	case pgtype.Float4OID, pgtype.Float8OID, pgtype.NumericOID:
		return "number", true
	case pgtype.TextOID, pgtype.VarcharOID, pgtype.BPCharOID:
		return "string", true
	case pgtype.UUIDOID:
		return "uuid", true
	case pgtype.TimestampOID, pgtype.TimestamptzOID, pgtype.DateOID, pgtype.IntervalOID:
		return "datetime", true
	default:
		return "", false
	}
}

func decodeValue(oid uint32, value []byte) (any, error) {
	switch oid {
	case pgtype.BoolOID:
		return string(value) == "t", nil
	case pgtype.Int2OID, pgtype.Int4OID, pgtype.Int8OID:
		parsed, err := strconv.ParseInt(string(value), 10, 64)
		if err != nil {
			return nil, &FailureError{Kind: "invalid integer result"}
		}
		return parsed, nil
	case pgtype.Float4OID, pgtype.Float8OID, pgtype.NumericOID:
		parsed, err := strconv.ParseFloat(string(value), 64)
		if err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) {
			return string(value), nil
		}
		return json.Number(string(value)), nil
	default:
		return string(value), nil
	}
}
