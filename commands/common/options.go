package common

type EditionOptions struct {
	MaxCodeChars               int `json:"maxCodeChars"`
	MaxVersionNumberChars      int `json:"maxVersionNumberChars"`
	MaxVersionCommitShaChars   int `json:"maxVersionCommitShaChars"`
	MaxVersionDescriptionChars int `json:"maxVersionDescriptionChars"`
}

type OptionsMetadata struct {
	Edition                                EditionOptions `json:"edition"`
	MinArtifactoryVersionForProjectSupport string         `json:"minArtifactoryVersionForProjectSupport"`
	IsTutorialAvailable                    bool           `json:"isTutorialAvailable"`
	IsFeedbackEnabled                      bool           `json:"isFeedbackEnabled"`
	IsHistoryEnabled                       bool           `json:"isHistoryEnabled"`
}
