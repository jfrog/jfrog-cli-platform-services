package common

//go:generate ${TOOLS_DIR}/mockgen -source=${GOFILE} -destination=mocks/${GOFILE}

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/robfig/cron/v3"
)

// ReadManifest reads a manifest from the working directory or from the directory provided as argument.
func ReadManifest(dir ...string) (*model.Manifest, error) {
	manifestFile, err := getManifestFile(dir...)
	if err != nil {
		return nil, err
	}

	log.Debug(fmt.Sprintf("Reading manifest from %s", manifestFile))

	manifestBytes, err := os.ReadFile(manifestFile)
	if err != nil {
		return nil, err
	}

	manifest := model.Manifest{}

	err = json.Unmarshal(manifestBytes, &manifest)
	if err != nil {
		return nil, err
	}

	return &manifest, nil
}

func getManifestFile(dir ...string) (string, error) {
	var manifestFolder string

	if len(dir) > 0 {
		manifestFolder = dir[0]
	} else {
		var err error
		if manifestFolder, err = os.Getwd(); err != nil {
			return "", err
		}
	}

	manifestFile := filepath.Join(manifestFolder, "manifest.json")

	return manifestFile, nil
}

func SaveManifest(mf *model.Manifest, dir ...string) error {
	manifestFile, err := getManifestFile(dir...)
	if err != nil {
		return err
	}

	writer, err := os.OpenFile(manifestFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}

	defer func() {
		closeErr := writer.Close()
		if closeErr != nil {
			if err == nil {
				err = errors.Join(err, closeErr)
			} else {
				err = closeErr
			}
		}
	}()

	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(mf)

	return err
}

// ReadSourceCode reads the content of the file pointed by SourceCodePath.
// The path must stay inside the current working directory (where manifest.json is read from), symlinks included.
func ReadSourceCode(mf *model.Manifest) (string, error) {
	log.Debug(fmt.Sprintf("Reading source code from %s", mf.SourceCodePath))

	if !filepath.IsLocal(mf.SourceCodePath) {
		return "", invalidManifestErr(fmt.Sprintf("sourceCodePath '%s' must be a relative path inside the worker directory", mf.SourceCodePath))
	}

	root, err := os.OpenRoot(".")
	if err != nil {
		return "", err
	}
	defer func() { _ = root.Close() }()

	sourceBytes, err := root.ReadFile(mf.SourceCodePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return "", invalidManifestErr(fmt.Sprintf("cannot read sourceCodePath '%s': %v", mf.SourceCodePath, err))
		}
		return "", err
	}
	return string(sourceBytes), nil
}

// workerKeyPattern mirrors the server rule (platform-extension, workerValidator.service.ts, assertWorkerKeyIsValid).
var workerKeyPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_\\-]+$`)

// ValidateWorkerKey checks the worker key against the same rules as the Workers service.
func ValidateWorkerKey(key string) error {
	switch {
	case len(key) < 2:
		return fmt.Errorf("invalid worker key '%s': must be at least 2 chars long", key)
	case len(key) > 40:
		return fmt.Errorf("invalid worker key '%s': must be lower than 40 chars long", key)
	case (key[0] < 'a' || key[0] > 'z') && (key[0] < 'A' || key[0] > 'Z'):
		return fmt.Errorf("invalid worker key '%s': must start with a letter", key)
	case !workerKeyPattern.MatchString(key):
		return fmt.Errorf("invalid worker key '%s'", key)
	}
	return nil
}

func ValidateManifest(mf *model.Manifest, actionsMeta ActionsMetadata) error {
	if mf.Name == "" {
		return invalidManifestErr("missing name")
	}

	if err := ValidateWorkerKey(mf.Name); err != nil {
		return invalidManifestErr(err.Error())
	}

	if mf.SourceCodePath == "" {
		return invalidManifestErr("missing source code path")
	}

	if mf.Action == "" {
		return invalidManifestErr("missing action")
	}

	if len(actionsMeta) > 0 {
		_, err := actionsMeta.FindAction(mf.Action, mf.Application)
		if err != nil {
			return invalidManifestErr(err.Error())
		}
	}

	return nil
}

func DecryptManifestSecrets(mf *model.Manifest, withPassword ...string) error {
	if len(mf.Secrets) == 0 {
		return nil
	}

	var password string
	if len(withPassword) > 0 {
		password = withPassword[0]
	} else {
		var err error
		password, err = ReadSecretPassword("Secrets Password: ")
		if err != nil {
			return err
		}
	}

	for name, value := range mf.Secrets {
		clearValue, err := DecryptSecret(password, value)
		if err != nil {
			log.Debug(fmt.Sprintf("cannot decrypt secret '%s': %+v", name, err))
			return fmt.Errorf("cannot decrypt secret '%s', please check the manifest", name)
		}
		mf.Secrets[name] = clearValue
	}

	return nil
}

func ValidateFilterCriteria(c *model.FilterCriteria, actionMeta *model.ActionMetadata) error {
	if !actionMeta.MandatoryFilter {
		return nil
	}

	if actionMeta.FilterType == model.FilterTypeSchedule {
		if c.ArtifactFilterCriteria != nil {
			return invalidManifestErr("scheduled event cannot have artifact filter criteria")
		}
		if err := ValidateScheduleCriteria(c.Schedule); err != nil {
			return fmt.Errorf("manifest validation failed: %w", err)
		}
	}

	if actionMeta.FilterType == model.FilterTypeRepo {
		if c.Schedule != nil {
			return invalidManifestErr("repository events cannot have schedule criteria")
		}
		if c.ArtifactFilterCriteria == nil {
			return invalidManifestErr("missing artifact filter criteria")
		}
		hasAnyRepo := c.ArtifactFilterCriteria.AnyFederated || c.ArtifactFilterCriteria.AnyLocal || c.ArtifactFilterCriteria.AnyRemote
		if len(c.ArtifactFilterCriteria.RepoKeys) == 0 && !hasAnyRepo {
			return invalidManifestErr("at least one repository key must be provided")
		}
	}

	return nil
}

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

func ValidateScheduleCriteria(c *model.ScheduleFilterCriteria) error {
	if c.Cron == "" {
		return errors.New("missing cron expression")
	}

	if _, err := cronParser.Parse(c.Cron); err != nil {
		log.Debug(fmt.Sprintf("invalid cron expression: %+v", err))
		return errors.New("invalid cron expression")
	}

	if c.Timezone != "" && !model.IsValidTimezone(c.Timezone) {
		return errors.New("invalid timezone '" + c.Timezone + "'")
	}

	return nil
}

func invalidManifestErr(reason string) error {
	return fmt.Errorf("invalid manifest: %s", reason)
}
