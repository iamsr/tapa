package models

// Phase represents the deployment phase when an alternative step should be executed
type Phase string

const (
	PhasePreDeploy    Phase = "PRE_DEPLOY"
	PhaseDuringDeploy Phase = "DURING_DEPLOY"
	PhasePostDeploy   Phase = "POST_DEPLOY"
	PhaseBackground   Phase = "BACKGROUND"
)

// AlternativeStep represents a single step in a multi-step alternative strategy
type AlternativeStep struct {
	StepNumber        int     `json:"step_number"`
	Phase             Phase   `json:"phase"`
	SQL               string  `json:"sql"`
	Description       string  `json:"description"`
	RequiresAppChange bool    `json:"requires_app_change"`
	RiskScore         int     `json:"risk_score"`
	EstimatedTime     float64 `json:"estimated_time"`
	CanRunOffline     bool    `json:"can_run_offline"`
}

// AlternativeStrategy represents a safer multi-step approach for a high-risk operation
type AlternativeStrategy struct {
	StrategyName  string            `json:"strategy_name"`
	Description   string            `json:"description"`
	Steps         []AlternativeStep `json:"steps"`
	RiskReduction int               `json:"risk_reduction"`
	Tradeoffs     []string          `json:"tradeoffs"`
	EstimatedTime float64           `json:"estimated_time"`
}
