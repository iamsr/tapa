package models

// DependencyType represents types of database dependencies
type DependencyType string

const (
	DependencyTypeIndex      DependencyType = "INDEX"
	DependencyTypeView       DependencyType = "VIEW"
	DependencyTypeForeignKey DependencyType = "FOREIGN_KEY"
	DependencyTypeConstraint DependencyType = "CONSTRAINT"
	DependencyTypeTrigger    DependencyType = "TRIGGER"
	DependencyTypeFunction   DependencyType = "FUNCTION"
)

// ImpactLevel represents severity of dependency impact
type ImpactLevel string

const (
	ImpactBreaks   ImpactLevel = "BREAKS"   // Dependency will be broken/dropped
	ImpactDegrades ImpactLevel = "DEGRADES" // Dependency will still work but performance degrades
	ImpactSafe     ImpactLevel = "SAFE"     // Dependency is safely handled
)

// Dependency represents something that depends on an operation's target
type Dependency struct {
	Type        DependencyType
	Name        string
	Definition  string // SQL definition (for views/functions)
	ImpactLevel ImpactLevel
	Description string // Human-readable impact explanation
}

// IsBreaking returns true if this dependency will break
func (d *Dependency) IsBreaking() bool {
	return d.ImpactLevel == ImpactBreaks
}
