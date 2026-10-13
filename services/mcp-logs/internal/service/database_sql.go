package service

import (
	"fmt"
	"strings"

	pg "github.com/pganalyze/pg_query_go/v6"
	parser "github.com/wasilibs/go-pgquery"

	"sandbox_url_shortener/services/mcp-logs/internal/domain"
)

// The parser is PostgreSQL's own grammar compiled to WASM. Unknown AST forms
// are denied rather than passed through a keyword filter.
type databaseSQLPolicy struct {
	parameters int
	used       map[int]bool
	remaining  int
}

func validateDatabaseSQL(sql string, parameters int) (string, error) {
	if strings.TrimSpace(sql) == "" || len(sql) > maxDatabaseSQLBytes {
		return "", fmt.Errorf("SQL must be nonempty and at most %d bytes", maxDatabaseSQLBytes)
	}
	parsed, err := parser.Parse(sql)
	if err != nil {
		return "", fmt.Errorf("invalid PostgreSQL syntax; SQL details withheld")
	}
	if len(parsed.Stmts) != 1 || parsed.Stmts[0].Stmt.GetSelectStmt() == nil {
		return "", fmt.Errorf("only one SELECT statement is allowed")
	}
	policy := databaseSQLPolicy{parameters: parameters, used: make(map[int]bool), remaining: 512}
	if err := policy.selectStatement(parsed.Stmts[0].Stmt.GetSelectStmt(), map[string]bool{}, 0); err != nil {
		return "", err
	}
	if len(policy.used) != parameters {
		return "", fmt.Errorf("bound parameters must match every positional placeholder without gaps")
	}
	canonical, err := parser.Deparse(parsed)
	if err != nil {
		return "", fmt.Errorf("unsupported PostgreSQL query form")
	}
	return canonical, nil
}

func (p *databaseSQLPolicy) selectStatement(selectStmt *pg.SelectStmt, inherited map[string]bool, depth int) error {
	if selectStmt == nil {
		return nil
	}
	if depth > 48 {
		return fmt.Errorf("SQL nesting limit exceeded")
	}
	if selectStmt.IntoClause != nil || len(selectStmt.LockingClause) > 0 {
		return fmt.Errorf("SELECT INTO and row locks are forbidden")
	}
	if len(selectStmt.WindowClause) > 0 || len(selectStmt.ValuesLists) > 0 {
		return fmt.Errorf("window clauses and VALUES are outside the supported SELECT grammar")
	}
	scope := make(map[string]bool, len(inherited))
	for name := range inherited {
		scope[name] = true
	}
	if err := p.withClause(selectStmt.WithClause, scope, depth); err != nil {
		return err
	}
	for _, nodes := range [][]*pg.Node{selectStmt.TargetList, selectStmt.FromClause, selectStmt.GroupClause, selectStmt.DistinctClause, selectStmt.SortClause} {
		if err := p.nodes(nodes, scope, depth+1); err != nil {
			return err
		}
	}
	for _, node := range []*pg.Node{selectStmt.WhereClause, selectStmt.HavingClause, selectStmt.LimitOffset, selectStmt.LimitCount} {
		if err := p.node(node, scope, depth+1); err != nil {
			return err
		}
	}
	if err := p.selectStatement(selectStmt.Larg, scope, depth+1); err != nil {
		return err
	}
	return p.selectStatement(selectStmt.Rarg, scope, depth+1)
}

func (p *databaseSQLPolicy) withClause(with *pg.WithClause, scope map[string]bool, depth int) error {
	if with != nil {
		if with.Recursive {
			return fmt.Errorf("recursive CTEs are forbidden")
		}
		for _, node := range with.Ctes {
			cte := node.GetCommonTableExpr()
			if cte == nil || cte.Ctequery.GetSelectStmt() == nil || cte.SearchClause != nil || cte.CycleClause != nil {
				return fmt.Errorf("only read-only CTEs are allowed")
			}
			if err := p.selectStatement(cte.Ctequery.GetSelectStmt(), scope, depth+1); err != nil {
				return err
			}
			scope[cte.Ctename] = true
		}
	}
	return nil
}

func (p *databaseSQLPolicy) nodes(nodes []*pg.Node, scope map[string]bool, depth int) error {
	for _, node := range nodes {
		if err := p.node(node, scope, depth); err != nil {
			return err
		}
	}
	return nil
}

func (p *databaseSQLPolicy) node(node *pg.Node, scope map[string]bool, depth int) error {
	if node == nil {
		return nil
	}
	p.remaining--
	if p.remaining < 0 || depth > 48 {
		return fmt.Errorf("SQL complexity limit exceeded")
	}
	switch node.Node.(type) {
	case *pg.Node_SelectStmt:
		return p.selectStatement(node.GetSelectStmt(), scope, depth+1)
	case *pg.Node_RangeVar, *pg.Node_RangeSubselect, *pg.Node_JoinExpr:
		return p.relationNode(node, scope, depth)
	case *pg.Node_ResTarget, *pg.Node_ColumnRef, *pg.Node_AConst, *pg.Node_ParamRef:
		return p.leafNode(node, scope, depth)
	case *pg.Node_AExpr, *pg.Node_BoolExpr, *pg.Node_FuncCall, *pg.Node_TypeCast:
		return p.callNode(node, scope, depth)
	case *pg.Node_NullTest, *pg.Node_BooleanTest, *pg.Node_CoalesceExpr, *pg.Node_CaseExpr, *pg.Node_CaseWhen:
		return p.conditionalNode(node, scope, depth)
	case *pg.Node_List, *pg.Node_SortBy, *pg.Node_SubLink:
		return p.collectionNode(node, scope, depth)
	default:
		return fmt.Errorf("SQL form is outside the supported read-only grammar")
	}
}

func (p *databaseSQLPolicy) relationNode(node *pg.Node, scope map[string]bool, depth int) error {
	switch value := node.Node.(type) {
	case *pg.Node_RangeVar:
		rangeVar := value.RangeVar
		if rangeVar.Catalogname != "" {
			return fmt.Errorf("cross-database relations are forbidden")
		}
		if rangeVar.Schemaname == "" && scope[rangeVar.Relname] {
			return nil
		}
		if rangeVar.Schemaname != domain.InvestigationSchema || rangeVar.Relname != domain.InvestigationLinksView {
			return fmt.Errorf("relation is not approved; use describe_database")
		}
		return nil
	case *pg.Node_RangeSubselect:
		if value.RangeSubselect.Subquery.GetSelectStmt() == nil {
			return fmt.Errorf("only SELECT subqueries are allowed")
		}
		return p.node(value.RangeSubselect.Subquery, scope, depth+1)
	case *pg.Node_JoinExpr:
		join := value.JoinExpr
		for _, child := range []*pg.Node{join.Larg, join.Rarg, join.Quals} {
			if err := p.node(child, scope, depth+1); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("SQL form is outside the supported read-only grammar")
	}
}

func (p *databaseSQLPolicy) leafNode(node *pg.Node, scope map[string]bool, depth int) error {
	switch value := node.Node.(type) {
	case *pg.Node_ResTarget:
		if len(value.ResTarget.Indirection) > 0 {
			return fmt.Errorf("target indirection is unsupported")
		}
		return p.node(value.ResTarget.Val, scope, depth+1)
	case *pg.Node_ColumnRef:
		for _, field := range value.ColumnRef.Fields {
			if field.GetString_() == nil {
				return fmt.Errorf("select explicit columns instead of star projections")
			}
		}
		return nil
	case *pg.Node_AConst:
		return nil
	case *pg.Node_ParamRef:
		number := int(value.ParamRef.Number)
		if number < 1 || number > p.parameters {
			return fmt.Errorf("placeholder does not match bound parameters")
		}
		p.used[number] = true
		return nil
	default:
		return fmt.Errorf("SQL form is outside the supported read-only grammar")
	}
}

func (p *databaseSQLPolicy) callNode(node *pg.Node, scope map[string]bool, depth int) error {
	switch value := node.Node.(type) {
	case *pg.Node_AExpr:
		if err := validateDatabaseOperator(value.AExpr.Name); err != nil {
			return err
		}
		switch value.AExpr.Kind {
		case pg.A_Expr_Kind_AEXPR_OP, pg.A_Expr_Kind_AEXPR_DISTINCT, pg.A_Expr_Kind_AEXPR_NOT_DISTINCT, pg.A_Expr_Kind_AEXPR_NULLIF, pg.A_Expr_Kind_AEXPR_IN, pg.A_Expr_Kind_AEXPR_LIKE, pg.A_Expr_Kind_AEXPR_ILIKE, pg.A_Expr_Kind_AEXPR_BETWEEN, pg.A_Expr_Kind_AEXPR_NOT_BETWEEN:
		default:
			return fmt.Errorf("operator form is outside the supported grammar")
		}
		if err := p.node(value.AExpr.Lexpr, scope, depth+1); err != nil {
			return err
		}
		return p.node(value.AExpr.Rexpr, scope, depth+1)
	case *pg.Node_BoolExpr:
		return p.nodes(value.BoolExpr.Args, scope, depth+1)
	case *pg.Node_FuncCall:
		return p.functionCall(value.FuncCall, scope, depth)
	case *pg.Node_TypeCast:
		typeName := value.TypeCast.TypeName
		name, ok := builtinDatabaseName(typeName.Names)
		if !ok || !safeDatabaseType(name) || typeName.Setof || typeName.PctType || len(typeName.ArrayBounds) > 0 || len(typeName.Typmods) > 0 {
			return fmt.Errorf("cast type is not approved")
		}
		typeName.Names = qualifiedDatabaseName(name)
		return p.node(value.TypeCast.Arg, scope, depth+1)
	default:
		return fmt.Errorf("SQL form is outside the supported read-only grammar")
	}
}

func (p *databaseSQLPolicy) functionCall(function *pg.FuncCall, scope map[string]bool, depth int) error {
	name, ok := builtinDatabaseName(function.Funcname)
	if !ok || !safeDatabaseFunction(name) || function.Over != nil || function.FuncVariadic || function.AggWithinGroup || len(function.AggOrder) > 0 {
		return fmt.Errorf("function is not approved for investigation")
	}
	// Explicit qualification prevents future search-path changes from
	// redirecting an audited built-in name to an application function.
	function.Funcname = qualifiedDatabaseName(name)
	if err := p.nodes(function.Args, scope, depth+1); err != nil {
		return err
	}
	return p.node(function.AggFilter, scope, depth+1)
}

func (p *databaseSQLPolicy) conditionalNode(node *pg.Node, scope map[string]bool, depth int) error {
	switch value := node.Node.(type) {
	case *pg.Node_NullTest:
		return p.node(value.NullTest.Arg, scope, depth+1)
	case *pg.Node_BooleanTest:
		return p.node(value.BooleanTest.Arg, scope, depth+1)
	case *pg.Node_CoalesceExpr:
		return p.nodes(value.CoalesceExpr.Args, scope, depth+1)
	case *pg.Node_CaseExpr:
		if err := p.node(value.CaseExpr.Arg, scope, depth+1); err != nil {
			return err
		}
		if err := p.nodes(value.CaseExpr.Args, scope, depth+1); err != nil {
			return err
		}
		return p.node(value.CaseExpr.Defresult, scope, depth+1)
	case *pg.Node_CaseWhen:
		if err := p.node(value.CaseWhen.Expr, scope, depth+1); err != nil {
			return err
		}
		return p.node(value.CaseWhen.Result, scope, depth+1)
	default:
		return fmt.Errorf("SQL form is outside the supported read-only grammar")
	}
}

func (p *databaseSQLPolicy) collectionNode(node *pg.Node, scope map[string]bool, depth int) error {
	switch value := node.Node.(type) {
	case *pg.Node_List:
		return p.nodes(value.List.Items, scope, depth+1)
	case *pg.Node_SortBy:
		if len(value.SortBy.UseOp) > 0 {
			return fmt.Errorf("custom sorting operators are forbidden")
		}
		return p.node(value.SortBy.Node, scope, depth+1)
	case *pg.Node_SubLink:
		link := value.SubLink
		if link.Subselect.GetSelectStmt() == nil || len(link.OperName) > 0 {
			return fmt.Errorf("subquery form is unsupported")
		}
		switch link.SubLinkType {
		case pg.SubLinkType_EXISTS_SUBLINK, pg.SubLinkType_EXPR_SUBLINK, pg.SubLinkType_ANY_SUBLINK:
		default:
			return fmt.Errorf("subquery form is unsupported")
		}
		if err := p.node(link.Testexpr, scope, depth+1); err != nil {
			return err
		}
		return p.node(link.Subselect, scope, depth+1)
	default:
		return fmt.Errorf("SQL form is outside the supported read-only grammar")
	}
}

func builtinDatabaseName(nodes []*pg.Node) (string, bool) {
	if len(nodes) == 1 && nodes[0].GetString_() != nil {
		return nodes[0].GetString_().Sval, true
	}
	if len(nodes) == 2 && nodes[0].GetString_().GetSval() == "pg_catalog" && nodes[1].GetString_() != nil {
		return nodes[1].GetString_().Sval, true
	}
	return "", false
}

func qualifiedDatabaseName(name string) []*pg.Node {
	return []*pg.Node{{Node: &pg.Node_String_{String_: &pg.String{Sval: "pg_catalog"}}}, {Node: &pg.Node_String_{String_: &pg.String{Sval: name}}}}
}

func validateDatabaseOperator(nodes []*pg.Node) error {
	if len(nodes) != 1 || nodes[0].GetString_() == nil {
		return fmt.Errorf("custom operators are forbidden")
	}
	switch nodes[0].GetString_().Sval {
	case "=", "<>", "!=", "<", ">", "<=", ">=", "+", "-", "*", "/", "%", "~~", "!~~", "~~*", "!~~*":
		return nil
	default:
		return fmt.Errorf("operator is not approved for investigation")
	}
}

func safeDatabaseFunction(name string) bool {
	switch name {
	case "count", "sum", "avg", "min", "max", "abs", "round", "lower", "upper", "length", "char_length", "date_trunc", "timezone":
		return true
	default:
		return false
	}
}

func safeDatabaseType(name string) bool {
	switch name {
	case "text", "uuid", "bool", "int2", "int4", "int8", "float4", "float8", "numeric", "timestamp", "timestamptz", "date", "interval":
		return true
	default:
		return false
	}
}
