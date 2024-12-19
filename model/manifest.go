package model

type ArtifactFilterCriteria struct {
	RepoKeys []string `json:"repoKeys,omitempty"`
}

type ScheduleFilterCriteria struct {
	Cron     string `json:"cron,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

type FilterCriteria struct {
	ArtifactFilterCriteria ArtifactFilterCriteria `json:"artifactFilterCriteria,omitempty"`
	Schedule               ScheduleFilterCriteria `json:"schedule,omitempty"`
}

type Secrets map[string]string

type Manifest struct {
	Name           string         `json:"name"`
	Description    string         `json:"description"`
	SourceCodePath string         `json:"sourceCodePath"`
	Action         string         `json:"action"`
	Enabled        bool           `json:"enabled"`
	Debug          bool           `json:"debug"`
	ProjectKey     string         `json:"projectKey"`
	Secrets        Secrets        `json:"secrets"`
	FilterCriteria FilterCriteria `json:"filterCriteria,omitempty"`
	Application    string         `json:"application,omitempty"`
}
