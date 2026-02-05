package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yourusername/dma/pkg/models"
)

func TestGetParser(t *testing.T) {
	tests := []struct {
		name    string
		dbType  string
		wantErr bool
	}{
		{
			name:    "postgresql supported",
			dbType:  "postgresql",
			wantErr: false,
		},
		{
			name:    "mysql not yet implemented",
			dbType:  "mysql",
			wantErr: true,
		},
		{
			name:    "unsupported database",
			dbType:  "oracle",
			wantErr: true,
		},
		{
			name:    "empty database type",
			dbType:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser, err := GetParser(tt.dbType)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, parser)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, parser)
			}
		})
	}
}

func TestParserInterface(t *testing.T) {
	// Test that Parser interface is implemented correctly
	parser, err := GetParser("postgresql")
	assert.NoError(t, err)
	assert.NotNil(t, parser)

	// Verify Parse method exists and returns correct types
	ops, err := parser.Parse("")
	assert.NoError(t, err)
	assert.NotNil(t, ops)
	assert.IsType(t, []*models.Operation{}, ops)
}
