package model

type Version struct {
	CommitSha   string `json:"commitSha"`
	Description string `json:"description"`
	Number      string `json:"versionNumber"`
}

func (v *Version) IsEmpty() bool {
	return v == nil || (v.CommitSha == "" && v.Description == "" && v.Number == "")
}
