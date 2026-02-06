package introspector

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetIntrospector(t *testing.T) {
	tests := []struct {
		name          string
		dbType        string
		connURL       string
		wantErr       bool
		errorContains string
	}{
		{"postgresql", "postgresql", "postgres://localhost/test", false, ""},
		{"mysql", "mysql", "mysql://localhost/test", false, ""},
		{"unsupported", "oracle", "oracle://localhost/test", true, "unsupported"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intro, err := GetIntrospector(tt.dbType, tt.connURL)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, intro)
				if tt.errorContains != "" {
					assert.True(t, strings.Contains(err.Error(), tt.errorContains),
						"expected error to contain '%s', got: %s", tt.errorContains, err.Error())
				}
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, intro)
			}
		})
	}
}
