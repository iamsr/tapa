package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/iamsr/dma/pkg/models"
)

func TestParser_Parse_AddColumn(t *testing.T) {
	p := NewParser()

	sql := "ALTER TABLE users ADD COLUMN email VARCHAR(255);"

	ops, err := p.Parse(sql)
	require.NoError(t, err)
	require.Len(t, ops, 1)

	op := ops[0]
	assert.Equal(t, models.OperationTypeAddColumn, op.Type)
	assert.Equal(t, "users", op.TableName)
	assert.Contains(t, op.SQL, "ADD COLUMN email")
}

func TestParser_Parse_DropColumn(t *testing.T) {
	p := NewParser()

	sql := "ALTER TABLE users DROP COLUMN email;"

	ops, err := p.Parse(sql)
	require.NoError(t, err)
	require.Len(t, ops, 1)

	op := ops[0]
	assert.Equal(t, models.OperationTypeDropColumn, op.Type)
	assert.Equal(t, "users", op.TableName)
}

func TestParser_Parse_AlterColumn(t *testing.T) {
	tests := []struct {
		name string
		sql  string
	}{
		{
			name: "alter column type",
			sql:  "ALTER TABLE users ALTER COLUMN email TYPE TEXT;",
		},
		{
			name: "set not null",
			sql:  "ALTER TABLE users ALTER COLUMN email SET NOT NULL;",
		},
		{
			name: "drop not null",
			sql:  "ALTER TABLE users ALTER COLUMN email DROP NOT NULL;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			ops, err := p.Parse(tt.sql)
			require.NoError(t, err)
			require.Len(t, ops, 1)

			op := ops[0]
			assert.Equal(t, models.OperationTypeAlterColumn, op.Type)
			assert.Equal(t, "users", op.TableName)
		})
	}
}

func TestParser_Parse_CreateIndex(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		tableName string
	}{
		{
			name:      "regular index",
			sql:       "CREATE INDEX idx_users_email ON users(email);",
			tableName: "users",
		},
		{
			name:      "concurrent index",
			sql:       "CREATE INDEX CONCURRENTLY idx_users_email ON users(email);",
			tableName: "users",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			ops, err := p.Parse(tt.sql)
			require.NoError(t, err)
			require.Len(t, ops, 1)

			op := ops[0]
			assert.Equal(t, models.OperationTypeCreateIndex, op.Type)
			assert.Equal(t, tt.tableName, op.TableName)
		})
	}
}

func TestParser_Parse_DropIndex(t *testing.T) {
	tests := []struct {
		name string
		sql  string
	}{
		{
			name: "regular drop",
			sql:  "DROP INDEX idx_users_email;",
		},
		{
			name: "concurrent drop",
			sql:  "DROP INDEX CONCURRENTLY idx_users_email;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewParser()
			ops, err := p.Parse(tt.sql)
			require.NoError(t, err)
			require.Len(t, ops, 1)

			op := ops[0]
			assert.Equal(t, models.OperationTypeDropIndex, op.Type)
		})
	}
}

func TestParser_Parse_CreateTable(t *testing.T) {
	p := NewParser()

	sql := `CREATE TABLE users (
		id SERIAL PRIMARY KEY,
		email VARCHAR(255) NOT NULL,
		created_at TIMESTAMP DEFAULT NOW()
	);`

	ops, err := p.Parse(sql)
	require.NoError(t, err)
	require.Len(t, ops, 1)

	op := ops[0]
	assert.Equal(t, models.OperationTypeCreateTable, op.Type)
	assert.Equal(t, "users", op.TableName)
}

func TestParser_Parse_DropTable(t *testing.T) {
	p := NewParser()

	sql := "DROP TABLE users;"

	ops, err := p.Parse(sql)
	require.NoError(t, err)
	require.Len(t, ops, 1)

	op := ops[0]
	assert.Equal(t, models.OperationTypeDropTable, op.Type)
}

func TestParser_Parse_MultipleStatements(t *testing.T) {
	p := NewParser()

	sql := `
		ALTER TABLE users ADD COLUMN email VARCHAR(255);
		CREATE INDEX idx_users_email ON users(email);
		ALTER TABLE posts DROP COLUMN author;
	`

	ops, err := p.Parse(sql)
	require.NoError(t, err)
	require.Len(t, ops, 3)

	assert.Equal(t, models.OperationTypeAddColumn, ops[0].Type)
	assert.Equal(t, "users", ops[0].TableName)

	assert.Equal(t, models.OperationTypeCreateIndex, ops[1].Type)
	assert.Equal(t, "users", ops[1].TableName)

	assert.Equal(t, models.OperationTypeDropColumn, ops[2].Type)
	assert.Equal(t, "posts", ops[2].TableName)
}

func TestParser_Parse_InvalidSQL(t *testing.T) {
	p := NewParser()

	sql := "INVALID SQL STATEMENT;"

	_, err := p.Parse(sql)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse SQL")
}

func TestParser_Parse_CommentsInSQL(t *testing.T) {
	p := NewParser()

	sql := `
		-- Add email column to users
		ALTER TABLE users ADD COLUMN email VARCHAR(255);
		
		/* Create index for email lookups */
		CREATE INDEX idx_users_email ON users(email);
	`

	ops, err := p.Parse(sql)
	require.NoError(t, err)
	require.Len(t, ops, 2)

	assert.Equal(t, models.OperationTypeAddColumn, ops[0].Type)
	assert.Equal(t, models.OperationTypeCreateIndex, ops[1].Type)
}

func TestParser_Parse_SchemaQualifiedTable(t *testing.T) {
	p := NewParser()

	sql := "ALTER TABLE public.users ADD COLUMN email VARCHAR(255);"

	ops, err := p.Parse(sql)
	require.NoError(t, err)
	require.Len(t, ops, 1)

	op := ops[0]
	assert.Equal(t, models.OperationTypeAddColumn, op.Type)
	assert.Equal(t, "public.users", op.TableName)
}

func TestParser_ParseFile(t *testing.T) {
	// Test ParseFile method exists and has correct signature
	p := NewParser()

	// We'll test with a non-existent file to verify error handling
	migration, err := p.ParseFile("/non/existent/file.sql")
	assert.Error(t, err)
	assert.Nil(t, migration)
	assert.Contains(t, err.Error(), "failed to read file")
}
