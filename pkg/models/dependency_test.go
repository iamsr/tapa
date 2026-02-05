package models

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDependency_IsBreaking(t *testing.T) {
	dep := &Dependency{
		Type:        DependencyTypeIndex,
		Name:        "idx_users_email",
		ImpactLevel: ImpactBreaks,
		Description: "Index will be dropped",
	}

	assert.True(t, dep.IsBreaking())
}

func TestDependency_IsBreaking_False(t *testing.T) {
	dep := &Dependency{
		Type:        DependencyTypeIndex,
		Name:        "idx_users_email",
		ImpactLevel: ImpactSafe,
		Description: "Index will be rebuilt",
	}

	assert.False(t, dep.IsBreaking())
}
