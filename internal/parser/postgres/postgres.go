package postgres

import (
	"fmt"
	"os"

	pg_query "github.com/pganalyze/pg_query_go/v5"
	"github.com/iamsr/dma/pkg/models"
)

// Parser handles PostgreSQL DDL parsing
type Parser struct{}

// NewParser creates a new PostgreSQL parser
func NewParser() *Parser {
	return &Parser{}
}

// Parse analyzes SQL and returns detected operations
func (p *Parser) Parse(sql string) ([]*models.Operation, error) {
	result, err := pg_query.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SQL: %w", err)
	}

	operations := make([]*models.Operation, 0)

	for _, stmt := range result.Stmts {
		op, err := p.parseStatement(stmt, sql)
		if err != nil {
			// Log but continue with other statements
			continue
		}
		if op != nil {
			operations = append(operations, op)
		}
	}

	return operations, nil
}

// ParseFile reads and parses a migration file
func (p *Parser) ParseFile(filePath string) (*models.Migration, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	migration := models.NewMigration(filePath)

	operations, err := p.Parse(string(content))
	if err != nil {
		return nil, err
	}

	for _, op := range operations {
		migration.AddOperation(op)
	}

	return migration, nil
}

// parseStatement converts a pg_query statement to an Operation
func (p *Parser) parseStatement(stmt *pg_query.RawStmt, originalSQL string) (*models.Operation, error) {
	node := stmt.Stmt

	switch n := node.Node.(type) {
	case *pg_query.Node_AlterTableStmt:
		return p.parseAlterTable(n.AlterTableStmt, originalSQL)
	case *pg_query.Node_CreateStmt:
		return p.parseCreateTable(n.CreateStmt, originalSQL)
	case *pg_query.Node_DropStmt:
		return p.parseDropStatement(n.DropStmt, originalSQL)
	case *pg_query.Node_IndexStmt:
		return p.parseCreateIndex(n.IndexStmt, originalSQL)
	default:
		// Not a DDL statement we care about
		return nil, nil
	}
}

// parseAlterTable handles ALTER TABLE statements
func (p *Parser) parseAlterTable(stmt *pg_query.AlterTableStmt, sql string) (*models.Operation, error) {
	op := &models.Operation{
		SQL:       sql,
		Type:      models.OperationTypeAlterTable,
		TableName: p.getRelationName(stmt.Relation),
	}

	// Determine specific operation type from subcommands
	if len(stmt.Cmds) > 0 {
		cmd := stmt.Cmds[0]
		switch c := cmd.Node.(type) {
		case *pg_query.Node_AlterTableCmd:
			switch c.AlterTableCmd.Subtype {
			case pg_query.AlterTableType_AT_AddColumn:
				op.Type = models.OperationTypeAddColumn
			case pg_query.AlterTableType_AT_DropColumn:
				op.Type = models.OperationTypeDropColumn
			case pg_query.AlterTableType_AT_AlterColumnType:
				op.Type = models.OperationTypeAlterColumn
			case pg_query.AlterTableType_AT_SetNotNull:
				op.Type = models.OperationTypeAlterColumn
			case pg_query.AlterTableType_AT_DropNotNull:
				op.Type = models.OperationTypeAlterColumn
			}
		}
	}

	return op, nil
}

// parseCreateTable handles CREATE TABLE statements
func (p *Parser) parseCreateTable(stmt *pg_query.CreateStmt, sql string) (*models.Operation, error) {
	return &models.Operation{
		SQL:       sql,
		Type:      models.OperationTypeCreateTable,
		TableName: p.getRelationName(stmt.Relation),
	}, nil
}

// parseDropStatement handles DROP statements
func (p *Parser) parseDropStatement(stmt *pg_query.DropStmt, sql string) (*models.Operation, error) {
	if len(stmt.Objects) == 0 {
		return nil, fmt.Errorf("DROP statement has no objects")
	}

	opType := models.OperationTypeUnknown
	switch stmt.RemoveType {
	case pg_query.ObjectType_OBJECT_TABLE:
		opType = models.OperationTypeDropTable
	case pg_query.ObjectType_OBJECT_INDEX:
		opType = models.OperationTypeDropIndex
	}

	return &models.Operation{
		SQL:  sql,
		Type: opType,
	}, nil
}

// parseCreateIndex handles CREATE INDEX statements
func (p *Parser) parseCreateIndex(stmt *pg_query.IndexStmt, sql string) (*models.Operation, error) {
	return &models.Operation{
		SQL:       sql,
		Type:      models.OperationTypeCreateIndex,
		TableName: p.getRelationName(stmt.Relation),
	}, nil
}

// getRelationName extracts table name from RangeVar
func (p *Parser) getRelationName(rel *pg_query.RangeVar) string {
	if rel == nil {
		return ""
	}

	if rel.Schemaname != "" {
		return rel.Schemaname + "." + rel.Relname
	}
	return rel.Relname
}
