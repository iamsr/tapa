package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTimeBreakdown_Total(t *testing.T) {
	tb := &TimeBreakdown{
		TableRewriteSeconds:    100.0,
		IndexBuildSeconds:      50.0,
		ConstraintCheckSeconds: 20.0,
		MetadataUpdateSeconds:  0.5,
	}

	tb.CalculateTotal()

	assert.Equal(t, 170.5, tb.TotalSeconds)
}

func TestTimeBreakdown_String(t *testing.T) {
	tb := &TimeBreakdown{
		TableRewriteSeconds: 100.0,
		IndexBuildSeconds:   50.0,
		TotalSeconds:        150.0,
	}

	str := tb.String()
	assert.Contains(t, str, "Table Rewrite: 100.0s")
	assert.Contains(t, str, "Index Build: 50.0s")
	assert.Contains(t, str, "Total: 150.0s")
}
