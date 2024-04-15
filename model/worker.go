package model

type WorkerDetails struct {
	Key            string         `json:"key"`
	Description    string         `json:"description"`
	Enabled        bool           `json:"enabled"`
	SourceCode     string         `json:"sourceCode"`
	Action         string         `json:"action"`
	FilterCriteria FilterCriteria `json:"filterCriteria,omitempty"`
	Secrets        []*Secret      `json:"secrets"`
}
