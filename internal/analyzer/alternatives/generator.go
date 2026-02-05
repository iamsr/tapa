package alternatives

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/yourusername/dma/pkg/models"
)

// AlternativeGenerator generates safer multi-step alternatives for high-risk operations
type AlternativeGenerator interface {
	// GenerateAlternatives returns alternative strategies for the given operation
	GenerateAlternatives(ctx context.Context, op *models.Operation) ([]models.AlternativeStrategy, error)

	// CanGenerateAlternative returns true if an alternative can be generated for this operation
	CanGenerateAlternative(op *models.Operation) bool
}

// GetAlternativeGenerator returns an alternative generator for the specified database type
func GetAlternativeGenerator(dbType string) (AlternativeGenerator, error) {
	normalizedDBType := strings.ToLower(dbType)

	switch normalizedDBType {
	case "postgresql", "postgres":
		return &postgresGenerator{}, nil
	case "mysql":
		return nil, fmt.Errorf("MySQL alternative generator not yet implemented")
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

// postgresGenerator generates alternatives for PostgreSQL operations
type postgresGenerator struct{}

// CanGenerateAlternative checks if an alternative can be generated for the operation
func (g *postgresGenerator) CanGenerateAlternative(op *models.Operation) bool {
	// Only generate alternatives for high-risk operations (>= 51)
	if op.RiskScore < 51 {
		return false
	}

	// Check if we support this operation type
	switch op.Type {
	case models.OperationTypeAddColumn:
		// Only if it has a DEFAULT value
		return g.hasDefaultValue(op.SQL)

	case models.OperationTypeCreateIndex:
		// Only if it doesn't already use CONCURRENTLY
		return !g.hasConcurrently(op.SQL)

	case models.OperationTypeAlterColumn:
		// Only if it's a type change
		return g.isTypeChange(op.SQL)

	default:
		return false
	}
}

// GenerateAlternatives generates alternative strategies for the given operation
func (g *postgresGenerator) GenerateAlternatives(ctx context.Context, op *models.Operation) ([]models.AlternativeStrategy, error) {
	// Return empty slice for operations that don't qualify
	if !g.CanGenerateAlternative(op) {
		return []models.AlternativeStrategy{}, nil
	}

	// Generate alternatives based on operation type
	switch op.Type {
	case models.OperationTypeAddColumn:
		if g.hasDefaultValue(op.SQL) {
			return []models.AlternativeStrategy{g.generateAddColumnAlternative(op)}, nil
		}

	case models.OperationTypeCreateIndex:
		if !g.hasConcurrently(op.SQL) {
			return []models.AlternativeStrategy{g.generateConcurrentIndexAlternative(op)}, nil
		}

	case models.OperationTypeAlterColumn:
		if g.isTypeChange(op.SQL) {
			return []models.AlternativeStrategy{g.generateAlterTypeAlternative(op)}, nil
		}
	}

	return []models.AlternativeStrategy{}, nil
}

// generateAddColumnAlternative creates a 3-step alternative for ADD COLUMN with DEFAULT
func (g *postgresGenerator) generateAddColumnAlternative(op *models.Operation) models.AlternativeStrategy {
	columnName := g.extractColumnName(op.SQL)
	columnType := g.extractColumnType(op.SQL)
	defaultValue := g.extractDefaultValue(op.SQL)

	// Step 1: ADD COLUMN without DEFAULT (fast, low risk)
	step1SQL := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", op.TableName, columnName, columnType)
	step1 := models.AlternativeStep{
		StepNumber:        1,
		Phase:             models.PhasePreDeploy,
		SQL:               step1SQL,
		Description:       "Add column without default value (fast operation)",
		RequiresAppChange: false,
		RiskScore:         15,
		EstimatedTime:     0.1,
		CanRunOffline:     false,
	}

	// Step 2: UPDATE to backfill (can run offline, medium risk)
	step2SQL := fmt.Sprintf("UPDATE %s SET %s = %s WHERE %s IS NULL", op.TableName, columnName, defaultValue, columnName)
	step2 := models.AlternativeStep{
		StepNumber:        2,
		Phase:             models.PhaseBackground,
		SQL:               step2SQL,
		Description:       "Backfill existing rows with default value",
		RequiresAppChange: false,
		RiskScore:         20,
		EstimatedTime:     op.EstimatedTimeSeconds * 0.5, // 50% of original time
		CanRunOffline:     true,
	}

	// Step 3: ALTER COLUMN SET DEFAULT (fast, low risk)
	step3SQL := fmt.Sprintf("ALTER TABLE %s ALTER COLUMN %s SET DEFAULT %s", op.TableName, columnName, defaultValue)
	step3 := models.AlternativeStep{
		StepNumber:        3,
		Phase:             models.PhaseDuringDeploy,
		SQL:               step3SQL,
		Description:       "Set default value for future inserts",
		RequiresAppChange: false,
		RiskScore:         10,
		EstimatedTime:     0.1,
		CanRunOffline:     false,
	}

	// Calculate total time
	totalTime := step1.EstimatedTime + step2.EstimatedTime + step3.EstimatedTime

	return models.AlternativeStrategy{
		StrategyName:  "Multi-step Column Addition",
		Description:   "Add column in three steps to avoid table rewrite",
		Steps:         []models.AlternativeStep{step1, step2, step3},
		RiskReduction: 50,
		Tradeoffs: []string{
			"Requires multiple deployments",
			"Column will be NULL initially",
			"Need to handle NULL in application code temporarily",
		},
		EstimatedTime: totalTime,
	}
}

// generateConcurrentIndexAlternative creates an alternative for CREATE INDEX with CONCURRENTLY
func (g *postgresGenerator) generateConcurrentIndexAlternative(op *models.Operation) models.AlternativeStrategy {
	// Use case-insensitive regex with word boundaries to avoid matching in comments/strings
	pattern := regexp.MustCompile(`(?i)\bCREATE\s+INDEX\b`)
	concurrentSQL := pattern.ReplaceAllString(op.SQL, "CREATE INDEX CONCURRENTLY")

	step := models.AlternativeStep{
		StepNumber:        1,
		Phase:             models.PhaseDuringDeploy,
		SQL:               concurrentSQL,
		Description:       "Create index without blocking writes",
		RequiresAppChange: false,
		RiskScore:         20,
		EstimatedTime:     op.EstimatedTimeSeconds * 1.5, // 150% of original time
		CanRunOffline:     false,
	}

	return models.AlternativeStrategy{
		StrategyName:  "Concurrent Index Creation",
		Description:   "Use CONCURRENTLY to avoid blocking writes",
		Steps:         []models.AlternativeStep{step},
		RiskReduction: 30,
		Tradeoffs: []string{
			"Takes longer than regular index creation",
			"Cannot run inside a transaction",
			"May fail and leave invalid index that needs cleanup",
		},
		EstimatedTime: step.EstimatedTime,
	}
}

// generateAlterTypeAlternative creates a simplified alternative for ALTER COLUMN TYPE
func (g *postgresGenerator) generateAlterTypeAlternative(op *models.Operation) models.AlternativeStrategy {
	// Simplified implementation with placeholder SQL
	step := models.AlternativeStep{
		StepNumber:        1,
		Phase:             models.PhaseDuringDeploy,
		SQL:               "-- Multi-step type change: add new column, copy data, rename (details depend on specific case)",
		Description:       "Placeholder for multi-column type change strategy",
		RequiresAppChange: true,
		RiskScore:         40,
		EstimatedTime:     op.EstimatedTimeSeconds,
		CanRunOffline:     false,
	}

	return models.AlternativeStrategy{
		StrategyName:  "Multi-column Type Change",
		Description:   "Add new column with desired type, copy data, rename",
		Steps:         []models.AlternativeStep{step},
		RiskReduction: 40,
		Tradeoffs: []string{
			"Requires multiple deployments",
			"Temporary storage increase (both columns exist)",
			"Application must be updated to use new column",
		},
		EstimatedTime: step.EstimatedTime,
	}
}

// Helper methods for SQL parsing

// hasDefaultValue checks if SQL contains a DEFAULT clause
func (g *postgresGenerator) hasDefaultValue(sql string) bool {
	// Use word boundaries to avoid matching "default" in column names
	pattern := regexp.MustCompile(`(?i)\bDEFAULT\b`)
	return pattern.MatchString(sql)
}

// hasConcurrently checks if SQL contains CONCURRENTLY keyword
func (g *postgresGenerator) hasConcurrently(sql string) bool {
	pattern := regexp.MustCompile(`(?i)\bCONCURRENTLY\b`)
	return pattern.MatchString(sql)
}

// isTypeChange checks if SQL is an ALTER COLUMN ... TYPE statement
func (g *postgresGenerator) isTypeChange(sql string) bool {
	pattern := regexp.MustCompile(`(?i)ALTER\s+COLUMN\s+\w+\s+TYPE\b`)
	return pattern.MatchString(sql)
}

// extractColumnName extracts column name from ADD COLUMN statement
func (g *postgresGenerator) extractColumnName(sql string) string {
	// Pattern: ADD COLUMN <name> <type>...
	// Support both quoted and unquoted identifiers
	pattern := regexp.MustCompile(`(?i)ADD\s+COLUMN\s+(?:"([^"]+)"|(\w+))`)
	matches := pattern.FindStringSubmatch(sql)
	if len(matches) > 2 {
		if matches[1] != "" {
			return matches[1] // quoted identifier
		}
		return matches[2] // unquoted identifier
	}
	return "column_name"
}

// extractColumnType extracts column type from ADD COLUMN statement
func (g *postgresGenerator) extractColumnType(sql string) string {
	// Pattern: ADD COLUMN <name> <type> [DEFAULT ...]
	// Allow commas, spaces in type parameters (e.g., NUMERIC(10,2))
	pattern := regexp.MustCompile(`(?i)ADD\s+COLUMN\s+\w+\s+([A-Z0-9(),\s]+?)(?:\s+(?:DEFAULT|NOT|NULL|UNIQUE|PRIMARY|CHECK|;|$))`)
	matches := pattern.FindStringSubmatch(sql)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return "TYPE"
}

// extractDefaultValue extracts default value from DEFAULT clause
func (g *postgresGenerator) extractDefaultValue(sql string) string {
	// Pattern: DEFAULT <value>
	// Match quoted strings (with internal quotes escaped) or unquoted values
	pattern := regexp.MustCompile(`(?i)DEFAULT\s+('(?:[^']|'')*'|"(?:[^"]|"")*"|[^\s,;]+)`)
	matches := pattern.FindStringSubmatch(sql)
	if len(matches) > 1 {
		return matches[1]
	}
	return "NULL"
}
