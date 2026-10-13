package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"

	"sandbox_url_shortener/services/mcp-logs/internal/service"
)

const (
	toolListDatabases    = "list_databases"
	toolDescribeDatabase = "describe_database"
	toolQueryDatabase    = "query_database"
)

func databaseDefinitions() []toolDefinition {
	return []toolDefinition{
		{Name: toolListDatabases, Description: "Lists configured database aliases without connecting to them or exposing credentials. Call before database investigation.", InputSchema: object(map[string]any{}, []string{}), Annotations: readOnlyTool()},
		{Name: toolDescribeDatabase, Description: "Lists only approved views and their diagnostic columns. Use before query_database. Credentials, original URLs and account data are excluded.", InputSchema: object(map[string]any{
			"database": stringProp("Alias returned by list_databases."),
		}, []string{"database"}), Annotations: readOnlyTool()},
		{Name: toolQueryDatabase, Description: "Runs one bounded read-only PostgreSQL SELECT over approved views. Supports joins, subqueries and nonrecursive read-only CTEs within the validated grammar. Use $1, $2 bind parameters; audited functions only. Results report truncation; a capped result has no exact total.", InputSchema: object(map[string]any{
			"database": stringProp("Alias returned by list_databases."),
			"sql":      stringProp("One SELECT using approved relations from describe_database."),
			"parameters": map[string]any{"type": "array", "maxItems": 32, "items": object(map[string]any{
				"type":  enumProp("Parameter type.", []string{"string", "int", "float", "bool", "null"}),
				"value": map[string]any{"type": []string{"string", "integer", "number", "boolean", "null"}},
			}, []string{"type", "value"})},
			"limit": intProp("Maximum returned rows; default 30, ceiling 100.", 1, 100),
		}, []string{"database", "sql"}), Annotations: readOnlyTool()},
	}
}

func (s *Server) dispatchDatabase(ctx context.Context, name string, raw json.RawMessage) (any, error) {
	if s.databases == nil {
		return nil, errors.New("database tools are unavailable")
	}
	switch name {
	case toolListDatabases:
		var a struct{}
		if err := decodeDatabaseArgs(raw, &a); err != nil {
			return nil, err
		}
		return s.databases.ListDatabases(ctx)
	case toolDescribeDatabase:
		var a struct {
			Database string `json:"database"`
		}
		if err := decodeDatabaseArgs(raw, &a); err != nil {
			return nil, err
		}
		return s.databases.DescribeDatabase(ctx, a.Database)
	default:
		var a struct {
			Database   string                      `json:"database"`
			SQL        string                      `json:"sql"`
			Parameters []service.DatabaseParameter `json:"parameters"`
			Limit      int                         `json:"limit"`
		}
		if err := decodeDatabaseArgs(raw, &a); err != nil {
			return nil, err
		}
		return s.databases.QueryDatabase(ctx, service.DatabaseQuery{Database: a.Database, SQL: a.SQL, Parameters: a.Parameters, Limit: a.Limit})
	}
}

// Database input errors must not echo SQL, parameters or caller-controlled field names.
func decodeDatabaseArgs(raw json.RawMessage, target any) error {
	if len(raw) == 0 {
		raw = json.RawMessage(`{}`)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	decoder.UseNumber()
	if err := decoder.Decode(target); err != nil {
		return errors.New("invalid database tool arguments")
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return errors.New("invalid database tool arguments")
	}
	return nil
}
