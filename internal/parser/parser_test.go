package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetParser(t *testing.T) {
	tests := []struct {
		name    string
		dbType  string
		wantErr bool
	}{
		{
			name:    "postgresql",
			dbType:  "postgresql",
			wantErr: false,
		},
		{
			name:    "mysql",
			dbType:  "mysql",
			wantErr: false,
		},
		{
			name:    "unsupported",
			dbType:  "oracle",
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
