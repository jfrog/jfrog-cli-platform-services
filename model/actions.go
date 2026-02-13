// Package model provides data structures for JFrog platform services.
package model

type ActionFilterType string

const (
	FilterTypeRepo     = "FILTER_REPO"
	FilterTypeSchedule = "SCHEDULE"
)

type Action struct {
	Application string `json:"application"`
	Name        string `json:"name"`
}

type ActionMetadata struct {
	Action               Action           `json:"action"`
	Description          string           `json:"description"`
	SamplePayload        string           `json:"samplePayload"`
	SampleCode           string           `json:"sampleCode"`
	TypesDefinitions     string           `json:"typesDefinitions"`
	SupportProjects      bool             `json:"supportProjects"`
	FilterType           ActionFilterType `json:"filterType"`
	MandatoryFilter      bool             `json:"mandatoryFilter"`
	WikiURL              string           `json:"wikiUrl"`
	Async                bool             `json:"async"`
	ExecutionRequestType string           `json:"executionRequestType"`
}
