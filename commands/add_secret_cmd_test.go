package commands

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

type addSecretAssertFunc func(t *testing.T, manifestBefore, manifestAfter *model.Manifest)

func TestAddSecretCmd(t *testing.T) {
	tests := []struct {
		name           string
		commandArgs    []string
		secretName     string
		secretValue    string
		secretPassword string
		wantErr        string
		assert         addSecretAssertFunc
		patchManifest  func(mf *model.Manifest)
	}{
		{
			name:           "add",
			secretName:     "sec-1",
			secretValue:    "val-1",
			secretPassword: secretPassword,
			patchManifest: func(mf *model.Manifest) {
				mf.Secrets = model.Secrets{
					"sec-2": mustEncryptSecret(t, "val-2"),
				}
			},
			assert: assertSecrets(model.Secrets{
				"sec-1": "val-1",
				"sec-2": "val-2",
			}),
		},
		{
			name:           "add with nil manifest",
			secretName:     "sec-1",
			secretValue:    "val-1",
			secretPassword: secretPassword,
			patchManifest: func(mf *model.Manifest) {
				mf.Secrets = nil
			},
			assert: assertSecrets(model.Secrets{
				"sec-1": "val-1",
			}),
		},
		{
			name:           "add with different password",
			secretName:     "sec-1",
			secretValue:    "val-1",
			secretPassword: secretPassword,
			patchManifest: func(mf *model.Manifest) {
				mf.Secrets["sec-2"] = mustEncryptSecret(t, "val-2", "other-password")
			},
			wantErr: "others secrets are encrypted with a different password, please use the same one",
		},
		{
			name:           "edit secret",
			secretName:     "sec-1",
			secretValue:    "val-1",
			secretPassword: secretPassword,
			commandArgs:    []string{fmt.Sprintf("--%s", model.FlagEdit)},
			patchManifest: func(mf *model.Manifest) {
				mf.Secrets = model.Secrets{
					"sec-1": mustEncryptSecret(t, "val-1-before"),
				}
			},
			assert: assertSecrets(model.Secrets{"sec-1": "val-1"}),
		},
		{
			name:           "fails if the secret exists",
			secretName:     "sec-1",
			secretValue:    "val-1",
			secretPassword: secretPassword,
			patchManifest: func(mf *model.Manifest) {
				mf.Secrets = model.Secrets{
					"sec-1": mustEncryptSecret(t, "val-1-before"),
				}
			},
			wantErr: "sec-1 already exists, use --edit to overwrite",
		},
		{
			name:    "fails if missing name",
			wantErr: "Wrong number of arguments (0).",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workerDir, workerName := prepareWorkerDirForTest(t)

			runCmd := createCliRunner(t, GetInitCommand(), GetAddSecretCommand())

			err := runCmd("worker", "init", "GENERIC_EVENT", workerName)
			require.NoError(t, err)

			if tt.patchManifest != nil {
				patchManifest(t, tt.patchManifest)
			}

			if tt.secretPassword != "" {
				err = os.Setenv(model.EnvKeySecretsPassword, tt.secretPassword)
				require.NoError(t, err)
				t.Cleanup(func() {
					_ = os.Unsetenv(model.EnvKeySecretsPassword)
				})
			}

			if tt.secretValue != "" {
				err = os.Setenv(model.EnvKeyAddSecretValue, tt.secretValue)
				require.NoError(t, err)
				t.Cleanup(func() {
					_ = os.Unsetenv(model.EnvKeyAddSecretValue)
				})
			}

			manifestBefore, err := model.ReadManifest(workerDir)
			require.NoError(t, err)

			cmd := []string{"worker", "add-secret"}
			cmd = append(cmd, tt.commandArgs...)

			if tt.secretName != "" {
				cmd = append(cmd, tt.secretName)
			}

			err = runCmd(cmd...)

			if tt.wantErr == "" {
				require.NoError(t, err)
				manifestAfter, err := model.ReadManifest(workerDir)
				assert.NoError(t, err)
				tt.assert(t, manifestBefore, manifestAfter)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}

func assertSecrets(wantSecrets model.Secrets) addSecretAssertFunc {
	return func(t *testing.T, manifestBefore, manifestAfter *model.Manifest) {
		require.Equalf(t, len(wantSecrets), len(manifestAfter.Secrets), "Invalid secrets length")
		require.NoError(t, manifestAfter.DecryptSecrets())
		assert.Equalf(t, wantSecrets, manifestAfter.Secrets, "Secrets mismatch")
	}
}
