package mysql

import (
	"fmt"
	"os"

	"github.com/yourusername/dma/pkg/models"
	"vitess.io/vitess/go/vt/sqlparser"
)

// Parser handles MySQL DDL parsing
type Parser struct{}

// NewParser creates a new MySQL parser
func NewParser() *Parser {
	return &Parser{}
}

// Parse analyzes SQL and returns detected operations
func (p *Parser) Parse(sql string) ([]*models.Operation, error) {
	parser := sqlparser.NewTestParser()
	stmt, err := parser.Parse(sql)
	if err != nil {
		return nil, fmt.Errorf("failed to parse SQL: %w", err)
	}

	var operations []*models.Operation

	switch stmt := stmt.(type) {
	case *sqlparser.CreateTable:
		op := &models.Operation{
			SQL:       sql,
			Type:      models.OperationTypeCreateTable,
			TableName: stmt.Table.Name.String(),
		}
		operations = append(operations, op)
	default:
		// Other statement types handled in subsequent tasks
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

	// Parse multiple statements
	parser := sqlparser.NewTestParser()
	statements, err := parser.SplitStatementToPieces(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to split statements: %w", err)
	}

	for _, stmt := range statements {
		if stmt == "" {
			continue
		}
		operations, err := p.Parse(stmt)
		if err != nil {
			return nil, err
		}
		for _, op := range operations {
			migration.AddOperation(op)
		}
	}

	return migration, nil
}
