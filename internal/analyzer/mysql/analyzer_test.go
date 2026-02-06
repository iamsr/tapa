package mysql

import (
	"context"
	"testing"

	"github.com/iamsr/dma/pkg/models"
)

func TestAnalyzer_AddColumn_LockDetection(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)
	ctx := context.Background()

	op := &models.Operation{
		Type:      models.OperationTypeAddColumn,
		TableName: "users",
		SQL:       "ALTER TABLE users ADD COLUMN status VARCHAR(50)",
	}

	err := analyzer.Analyze(ctx, op)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// ADD COLUMN without DEFAULT uses INPLACE algorithm, no lock
	if op.LockType != models.LockTypeNone {
		t.Errorf("Expected LockType NONE, got %s", op.LockType)
	}

	if op.RequiresRewrite {
		t.Errorf("Expected RequiresRewrite false, got true")
	}
}

func TestAnalyzer_AddColumn_WithDefault_LockDetection(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)
	ctx := context.Background()

	op := &models.Operation{
		Type:      models.OperationTypeAddColumn,
		TableName: "users",
		SQL:       "ALTER TABLE users ADD COLUMN status VARCHAR(50) DEFAULT 'active'",
	}

	err := analyzer.Analyze(ctx, op)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// ADD COLUMN with DEFAULT requires COPY algorithm (MySQL 5.7) or INSTANT (8.0+)
	// For safety, we assume it requires exclusive lock and rewrite
	if op.LockType != models.LockTypeExclusive {
		t.Errorf("Expected LockType EXCLUSIVE, got %s", op.LockType)
	}

	if !op.RequiresRewrite {
		t.Errorf("Expected RequiresRewrite true, got false")
	}
}

func TestAnalyzer_DropColumn(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)
	op := &models.Operation{
		Type:       models.OperationTypeDropColumn,
		TableName:  "users",
		ColumnName: "email",
		SQL:        "ALTER TABLE users DROP COLUMN email;",
	}
	err := analyzer.Analyze(context.Background(), op)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if op.LockType != models.LockTypeNone {
		t.Errorf("Expected NONE lock for DROP COLUMN, got %s", op.LockType)
	}
	if op.RequiresRewrite {
		t.Error("DROP COLUMN should not require rewrite")
	}
}

func TestAnalyzer_AlterColumn(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)
	op := &models.Operation{
		Type:       models.OperationTypeAlterColumn,
		TableName:  "users",
		ColumnName: "email",
		SQL:        "ALTER TABLE users MODIFY COLUMN email TEXT;",
	}
	err := analyzer.Analyze(context.Background(), op)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if op.LockType != models.LockTypeExclusive {
		t.Errorf("Expected EXCLUSIVE lock for ALTER COLUMN, got %s", op.LockType)
	}
	if !op.RequiresRewrite {
		t.Error("ALTER COLUMN should require rewrite")
	}
}

func TestAnalyzer_CreateIndex(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)
	op := &models.Operation{
		Type:      models.OperationTypeCreateIndex,
		TableName: "users",
		IndexName: "idx_email",
		SQL:       "CREATE INDEX idx_email ON users(email);",
	}
	err := analyzer.Analyze(context.Background(), op)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if op.LockType != models.LockTypeNone {
		t.Errorf("Expected NONE lock for CREATE INDEX, got %s", op.LockType)
	}
	if op.RequiresRewrite {
		t.Error("CREATE INDEX should not require rewrite with INPLACE")
	}
}

func TestAnalyzer_DropIndex(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)
	op := &models.Operation{
		Type:      models.OperationTypeDropIndex,
		TableName: "users",
		IndexName: "idx_email",
		SQL:       "DROP INDEX idx_email ON users;",
	}
	err := analyzer.Analyze(context.Background(), op)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if op.LockType != models.LockTypeNone {
		t.Errorf("Expected NONE lock for DROP INDEX, got %s", op.LockType)
	}
}

func TestAnalyzer_CreateTable(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)
	op := &models.Operation{
		Type:      models.OperationTypeCreateTable,
		TableName: "new_table",
		SQL:       "CREATE TABLE new_table (id INT PRIMARY KEY);",
	}
	err := analyzer.Analyze(context.Background(), op)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if op.LockType != models.LockTypeNone {
		t.Errorf("Expected NONE lock for CREATE TABLE, got %s", op.LockType)
	}
	if op.RiskScore != 10 {
		t.Errorf("Expected 10 risk score for CREATE TABLE, got %d", op.RiskScore)
	}
}

func TestAnalyzer_DropTable(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)
	op := &models.Operation{
		Type:      models.OperationTypeDropTable,
		TableName: "old_table",
		SQL:       "DROP TABLE old_table;",
	}
	err := analyzer.Analyze(context.Background(), op)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if op.LockType != models.LockTypeExclusive {
		t.Errorf("Expected EXCLUSIVE lock for DROP TABLE, got %s", op.LockType)
	}
	if op.RiskScore != 50 {
		t.Errorf("Expected 50 risk score for DROP TABLE, got %d", op.RiskScore)
	}
}

func TestAnalyzer_AlgorithmDetection(t *testing.T) {
	tests := []struct {
		sql      string
		expected string
	}{
		{"ALTER TABLE users ADD COLUMN x INT, ALGORITHM=INPLACE;", "INPLACE"},
		{"ALTER TABLE users ADD COLUMN x INT, ALGORITHM=COPY;", "COPY"},
		{"ALTER TABLE users ADD COLUMN x INT, ALGORITHM=INSTANT;", "INSTANT"},
		{"ALTER TABLE users ADD COLUMN x INT, ALGORITHM='INPLACE';", "INPLACE"},
		{"ALTER TABLE users ADD COLUMN x INT, ALGORITHM='COPY';", "COPY"},
		{"ALTER TABLE users ADD COLUMN x INT;", "DEFAULT"},
	}

	analyzer := NewAnalyzer(nil, 100, 2.0)
	for _, tt := range tests {
		result := analyzer.detectAlgorithm(tt.sql)
		if result != tt.expected {
			t.Errorf("detectAlgorithm(%q) = %q, want %q", tt.sql, result, tt.expected)
		}
	}
}

func TestAnalyzer_LockDetection(t *testing.T) {
	tests := []struct {
		sql      string
		expected string
	}{
		{"ALTER TABLE users ADD COLUMN x INT, LOCK=NONE;", "NONE"},
		{"ALTER TABLE users ADD COLUMN x INT, LOCK=SHARED;", "SHARED"},
		{"ALTER TABLE users ADD COLUMN x INT, LOCK=EXCLUSIVE;", "EXCLUSIVE"},
		{"ALTER TABLE users ADD COLUMN x INT, LOCK='NONE';", "NONE"},
		{"ALTER TABLE users ADD COLUMN x INT, LOCK='SHARED';", "SHARED"},
		{"ALTER TABLE users ADD COLUMN x INT;", "DEFAULT"},
	}

	analyzer := NewAnalyzer(nil, 100, 2.0)
	for _, tt := range tests {
		result := analyzer.detectLock(tt.sql)
		if result != tt.expected {
			t.Errorf("detectLock(%q) = %q, want %q", tt.sql, result, tt.expected)
		}
	}
}

func TestAnalyzer_AddColumn_WithAlgorithm(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)
	op := &models.Operation{
		Type:      models.OperationTypeAddColumn,
		TableName: "users",
		SQL:       "ALTER TABLE users ADD COLUMN x INT, ALGORITHM=COPY;",
	}
	err := analyzer.Analyze(context.Background(), op)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if op.LockType != models.LockTypeExclusive {
		t.Errorf("Expected EXCLUSIVE lock for ALGORITHM=COPY, got %s", op.LockType)
	}
	if !op.RequiresRewrite {
		t.Error("ALGORITHM=COPY should require rewrite")
	}
}

func TestAnalyzer_AddColumn_WithLock(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)
	op := &models.Operation{
		Type:      models.OperationTypeAddColumn,
		TableName: "users",
		SQL:       "ALTER TABLE users ADD COLUMN x INT, LOCK=SHARED;",
	}
	err := analyzer.Analyze(context.Background(), op)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if op.LockType != models.LockTypeShare {
		t.Errorf("Expected SHARE lock for LOCK=SHARED, got %s", op.LockType)
	}
}

func TestAnalyzer_DropColumn_WithCopy(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)
	op := &models.Operation{
		Type:      models.OperationTypeDropColumn,
		TableName: "users",
		SQL:       "ALTER TABLE users DROP COLUMN email, ALGORITHM=COPY;",
	}
	err := analyzer.Analyze(context.Background(), op)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if op.LockType != models.LockTypeExclusive {
		t.Errorf("Expected EXCLUSIVE lock for ALGORITHM=COPY, got %s", op.LockType)
	}
	if !op.RequiresRewrite {
		t.Error("ALGORITHM=COPY should require rewrite")
	}
}

func TestAnalyzer_CreateIndex_WithCopy(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)
	op := &models.Operation{
		Type:      models.OperationTypeCreateIndex,
		TableName: "users",
		IndexName: "idx_email",
		SQL:       "CREATE INDEX idx_email ON users(email), ALGORITHM=COPY;",
	}
	err := analyzer.Analyze(context.Background(), op)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if op.LockType != models.LockTypeExclusive {
		t.Errorf("Expected EXCLUSIVE lock for ALGORITHM=COPY, got %s", op.LockType)
	}
	if !op.RequiresRewrite {
		t.Error("ALGORITHM=COPY should require rewrite")
	}
}

func TestAnalyzer_DropIndex_WithCopy(t *testing.T) {
	analyzer := NewAnalyzer(nil, 100, 2.0)
	op := &models.Operation{
		Type:      models.OperationTypeDropIndex,
		TableName: "users",
		IndexName: "idx_email",
		SQL:       "DROP INDEX idx_email ON users, ALGORITHM=COPY;",
	}
	err := analyzer.Analyze(context.Background(), op)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}
	if op.LockType != models.LockTypeExclusive {
		t.Errorf("Expected EXCLUSIVE lock for ALGORITHM=COPY, got %s", op.LockType)
	}
	if !op.RequiresRewrite {
		t.Error("ALGORITHM=COPY should require rewrite")
	}
}
