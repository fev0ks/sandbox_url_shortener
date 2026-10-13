package domain

import "context"

// Approved PostgreSQL relation names are shared query authorization contracts.
const (
	InvestigationSchema    = "mcp_investigation"
	InvestigationLinksView = "links"
)

// DatabaseReader reads a validated query inside a repository-owned transaction.
type DatabaseReader interface {
	Query(ctx context.Context, sql string, parameters []any, maxRows int) (DatabasePage, error)
}

// DatabaseList exposes configured aliases without connection credentials.
type DatabaseList struct {
	Databases []string `json:"databases"`
	NextStep  string   `json:"next_step"`
}

// DatabaseColumn describes one approved or returned column.
type DatabaseColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// DatabaseRelation describes a relation approved for investigation.
type DatabaseRelation struct {
	Schema  string           `json:"schema"`
	Name    string           `json:"name"`
	Columns []DatabaseColumn `json:"columns"`
}

// DatabaseDescription lists approved relations of one configured database.
type DatabaseDescription struct {
	Database  string             `json:"database"`
	Relations []DatabaseRelation `json:"relations"`
	NextStep  string             `json:"next_step"`
}

// DatabasePage is a bounded repository result before model-facing sanitization.
type DatabasePage struct {
	Columns        []DatabaseColumn
	Rows           [][]any
	BytesTruncated bool
	ObservedRows   int
}

// DatabaseTruncation describes a bounded query without inventing a full count.
type DatabaseTruncation struct {
	Returned     int  `json:"returned"`
	Total        *int `json:"total,omitempty"`
	TotalAtLeast int  `json:"total_at_least,omitempty"`
	Capped       bool `json:"capped"`
	Rows         bool `json:"rows_truncated"`
	Bytes        bool `json:"bytes_truncated"`
	Fields       int  `json:"fields_truncated"`
}

// DatabaseResult carries ordered rows so duplicate column aliases remain safe.
type DatabaseResult struct {
	Database   string             `json:"database"`
	Columns    []DatabaseColumn   `json:"columns"`
	Rows       [][]any            `json:"rows"`
	Truncation DatabaseTruncation `json:"truncation"`
}
