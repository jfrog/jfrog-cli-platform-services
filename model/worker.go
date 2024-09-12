package model

import "strings"

type WorkerDetails struct {
	Key            string         `json:"key"`
	Description    string         `json:"description"`
	Debug          bool           `json:"debug"`
	Enabled        bool           `json:"enabled"`
	SourceCode     string         `json:"sourceCode"`
	Action         string         `json:"action"`
	FilterCriteria FilterCriteria `json:"filterCriteria,omitempty"`
	Secrets        []*Secret      `json:"secrets"`
	ProjectKey     string         `json:"projectKey"`
}

func (w *WorkerDetails) KeyWithProject() string {
	projectKey := strings.TrimSpace(w.ProjectKey)
	if projectKey != "" {
		projectPrefix := projectKey + "-"
		if !strings.HasPrefix(w.Key, projectPrefix) {
			return projectPrefix + w.Key
		}
	}
	return w.Key
}
