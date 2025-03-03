package model

type Secret struct {
	Key              string `json:"key"`
	Value            string `json:"value"`
	MarkedForRemoval bool   `json:"markedForRemoval"`
}
type WorkerDetails struct {
	Key            string          `json:"key"`
	Description    string          `json:"description"`
	Debug          bool            `json:"debug"`
	Enabled        bool            `json:"enabled"`
	SourceCode     string          `json:"sourceCode"`
	Action         string          `json:"action"`
	FilterCriteria *FilterCriteria `json:"filterCriteria,omitempty"`
	Secrets        []*Secret       `json:"secrets"`
	ProjectKey     string          `json:"projectKey"`
}
