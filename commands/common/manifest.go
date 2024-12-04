package common

//go:generate mockgen -source=${GOFILE} -destination=mocks/${GOFILE}

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/jfrog/jfrog-client-go/utils/log"
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

// ReadSourceCode reads the content of the file pointed by SourceCodePath
func ReadSourceCode(mf *model.Manifest) (string, error) {
	log.Debug(fmt.Sprintf("Reading source code from %s", mf.SourceCodePath))
	sourceBytes, err := os.ReadFile(mf.SourceCodePath)
	if err != nil {
		return "", err
	}
	return string(sourceBytes), nil
}

func ValidateManifest(mf *model.Manifest, actionsMeta ActionsMetadata) error {
	if mf.Name == "" {
		return invalidManifestErr("missing name")
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

func invalidManifestErr(reason string) error {
	return fmt.Errorf("invalid manifest: %s", reason)
}
