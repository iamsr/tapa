package models

type BatchResult struct {
	Strategy     *BatchingStrategy `json:"strategy"`
	DatabaseType string            `json:"database_type"`
}
