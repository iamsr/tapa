package estimator

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTimeEstimator(t *testing.T) {
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
			estimator, err := GetTimeEstimator(tt.dbType, nil, 200, 2.0)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, estimator)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, estimator)
			}
		})
	}
}
