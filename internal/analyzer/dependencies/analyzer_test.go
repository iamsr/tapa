package dependencies

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDependencyAnalyzer(t *testing.T) {
	tests := []struct {
		name    string
		dbType  string
		wantErr bool
	}{
		{"postgresql", "postgresql", false},
		{"mysql", "mysql", true},
		{"unsupported", "oracle", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			analyzer, err := GetDependencyAnalyzer(tt.dbType, nil)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, analyzer)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, analyzer)
			}
		})
	}
}
