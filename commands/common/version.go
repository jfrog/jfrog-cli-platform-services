package common

import (
	"fmt"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

func ValidateVersion(version *model.Version, options *OptionsMetadata) error {
	if len(version.Number) > options.Edition.MaxVersionNumberChars {
		return fmt.Errorf("version number exceeds maximum length of %d characters", options.Edition.MaxVersionNumberChars)
	}
	if len(version.CommitSha) > options.Edition.MaxVersionCommitShaChars {
		return fmt.Errorf("commit sha exceeds maximum length of %d characters", options.Edition.MaxVersionCommitShaChars)
	}
	if len(version.Description) > options.Edition.MaxVersionDescriptionChars {
		return fmt.Errorf("description exceeds maximum length of %d characters", options.Edition.MaxVersionDescriptionChars)
	}
	return nil
}
