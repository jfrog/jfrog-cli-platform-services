package common

//go:generate ${TOOLS_DIR}/mockgen -source=${GOFILE} -destination=mocks/${GOFILE}

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

func PrettifyJson(in []byte) []byte {
	var out bytes.Buffer
	if err := json.Indent(&out, in, "", "  "); err != nil {
		return in
	}
	return out.Bytes()
}

type stringFlagAware interface {
	GetStringFlagValue(string) string
}

// ExtractProjectAndKeyFromCommandContext Extracts the project key and worker key from the command context. If the project key is not provided, it will be taken from the manifest.
// The workerKey could either be the first argument or the name in the manifest.
// The first argument will only be considered as the workerKey if total arguments are greater than minArgument.
func ExtractProjectAndKeyFromCommandContext(c stringFlagAware, args []string, minArguments int, onlyGeneric bool) (string, string, error) {
	var workerKey string

	projectKey := c.GetStringFlagValue(model.FlagProjectKey)

	if len(args) > 0 && len(args) > minArguments {
		workerKey = args[0]
	}

	if workerKey == "" || projectKey == "" {
		manifest, err := ReadManifest()
		if err != nil {
			if workerKey == "" {
				return "", "", err
			}
			return workerKey, projectKey, nil
		}

		if err = ValidateManifest(manifest, nil); err != nil {
			return "", "", err
		}

		if onlyGeneric && manifest.Action != "GENERIC_EVENT" {
			return "", "", fmt.Errorf("only the GENERIC_EVENT actions are executable. Got %s", manifest.Action)
		}

		if workerKey == "" {
			workerKey = manifest.Name
		}

		if projectKey == "" {
			projectKey = manifest.ProjectKey
		}
	}

	return workerKey, projectKey, nil
}

func PrepareSecretsUpdate(mf *model.Manifest, existingWorker *model.WorkerDetails) []*model.Secret {
	// We will detect removed secrets
	removedSecrets := map[string]any{}
	if existingWorker != nil {
		for _, existingSecret := range existingWorker.Secrets {
			removedSecrets[existingSecret.Key] = struct{}{}
		}
	}

	var secrets []*model.Secret

	// Secrets should have already been decoded
	for secretName, secretValue := range mf.Secrets {
		_, secretExists := removedSecrets[secretName]
		if secretExists {
			// To take into account the local value of a secret
			secrets = append(secrets, &model.Secret{Key: secretName, MarkedForRemoval: true})
		}
		delete(removedSecrets, secretName)
		secrets = append(secrets, &model.Secret{Key: secretName, Value: secretValue})
	}

	for removedSecret := range removedSecrets {
		secrets = append(secrets, &model.Secret{Key: removedSecret, MarkedForRemoval: true})
	}

	return secrets
}
