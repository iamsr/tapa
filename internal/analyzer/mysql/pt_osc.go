package mysql

import (
	"fmt"
	"strings"

	"github.com/yourusername/dma/pkg/models"
)

// GeneratePtOscCommand generates pt-online-schema-change command for high-risk operations
func GeneratePtOscCommand(op *models.Operation, host, database string) string {
	if op.RiskScore < 50 {
		return "" // Not needed for low/medium risk
	}

	// Extract ALTER clause from SQL
	alterClause := extractAlterClause(op.SQL)
	if alterClause == "" {
		return ""
	}

	cmd := fmt.Sprintf("pt-online-schema-change --alter \"%s\" "+
		"--host=%s "+
		"--user=root "+
		"D=%s,t=%s "+
		"--execute",
		alterClause,
		host,
		database,
		op.TableName,
	)

	return cmd
}

// extractAlterClause extracts the ALTER portion from ALTER TABLE statement
func extractAlterClause(sql string) string {
	// Remove "ALTER TABLE table_name " prefix
	upper := strings.ToUpper(sql)
	idx := strings.Index(upper, "ALTER TABLE")
	if idx == -1 {
		return ""
	}

	// Check if there's content after "ALTER TABLE"
	afterAlterIdx := idx + len("ALTER TABLE ")
	if afterAlterIdx >= len(sql) {
		return ""
	}

	// Find the table name end
	afterAlter := sql[afterAlterIdx:]
	parts := strings.Fields(afterAlter)
	if len(parts) < 2 {
		return ""
	}

	// Skip table name, get the rest
	tableName := parts[0]
	remaining := strings.TrimPrefix(afterAlter, tableName)
	remaining = strings.TrimSpace(remaining)
	remaining = strings.TrimSuffix(remaining, ";")

	return remaining
}

// ShouldUsePtOsc determines if pt-osc is recommended
func ShouldUsePtOsc(op *models.Operation) bool {
	// Recommend pt-osc for high-risk operations that require table copy
	if op.RiskScore < 50 {
		return false
	}

	if !op.RequiresRewrite {
		return false
	}

	// Only for ALTER TABLE operations
	switch op.Type {
	case models.OperationTypeAddColumn,
		models.OperationTypeDropColumn,
		models.OperationTypeAlterColumn:
		return true
	default:
		return false
	}
}
