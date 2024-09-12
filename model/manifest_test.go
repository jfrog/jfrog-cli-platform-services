package model

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var manifestSample = &Manifest{
	Name:           "my-worker",
	Description:    "my worker description",
	SourceCodePath: "./my-worker.ts",
	Action:         "BEFORE_DOWNLOAD",
	Enabled:        true,
	Debug:          true,
	ProjectKey:     "a-project",
	Secrets: Secrets{
		"hidden1": "hidden1.value",
		"hidden2": "hidden2.value",
	},
	FilterCriteria: FilterCriteria{
		ArtifactFilterCriteria: ArtifactFilterCriteria{
			RepoKeys: []string{
				"my-repo-local",
			},
		},
	},
}

func TestReadManifest(t *testing.T) {
	tests := []struct {
		name     string
		dirAsArg bool
		sample   *Manifest
		assert   func(t *testing.T, mf *Manifest, readErr error)
	}{
		{
			name:   "in current dir",
			sample: manifestSample,
			assert: func(t *testing.T, mf *Manifest, readErr error) {
				require.NoError(t, readErr)
				assert.Equal(t, manifestSample, mf)
			},
		},
		{
			name:     "with dir as argument",
			sample:   manifestSample,
			dirAsArg: true,
			assert: func(t *testing.T, mf *Manifest, readErr error) {
				require.NoError(t, readErr)
				assert.Equal(t, manifestSample, mf)
			},
		},
		{
			name: "with missing manifest",
			assert: func(t *testing.T, mf *Manifest, readErr error) {
				require.Error(t, readErr)
				require.True(t, os.IsNotExist(readErr))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifestFolder, err := os.MkdirTemp("", "wks-cli-*.manifest")
			require.NoError(t, err)

			t.Cleanup(func() {
				// We do not care about this error
				_ = os.RemoveAll(manifestFolder)
			})

			if tt.sample != nil {
				manifestBytes, err := json.Marshal(tt.sample)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(manifestFolder, "manifest.json"), manifestBytes, os.ModePerm)
				require.NoError(t, err)
			}

			var dirParams []string
			if tt.dirAsArg {
				dirParams = append(dirParams, manifestFolder)
			} else {
				err = os.Chdir(manifestFolder)
				require.NoError(t, err)
			}

			mf, err := ReadManifest(dirParams...)

			tt.assert(t, mf, err)
		})
	}
}

func TestManifest_ReadSourceCode(t *testing.T) {
	tests := []struct {
		name       string
		sourceCode string
		manifest   *Manifest
		want       string
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name:       "nominal case",
			manifest:   manifestSample,
			sourceCode: "export async () => ({ status: 'SUCCESS' })",
			want:       "export async () => ({ status: 'SUCCESS' })",
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				assert.NoError(t, err)
				return err == nil
			},
		},
		{
			name:     "missing source file",
			manifest: manifestSample,
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				assert.Error(t, err)
				assert.True(t, os.IsNotExist(err))
				return false
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifestFolder, err := os.MkdirTemp("", "wks-cli-*.source")
			require.NoError(t, err)

			t.Cleanup(func() {
				// We do not care about this error
				_ = os.RemoveAll(manifestFolder)
			})

			if tt.sourceCode != "" {
				err = os.WriteFile(filepath.Join(manifestFolder, tt.manifest.SourceCodePath), []byte(tt.sourceCode), os.ModePerm)
				require.NoError(t, err)
			}

			err = os.Chdir(manifestFolder)
			require.NoError(t, err)

			got, err := manifestSample.ReadSourceCode()
			if !tt.wantErr(t, err, "ReadSourceCode()") {
				return
			}

			assert.Equalf(t, tt.want, got, "ReadSourceCode()")
		})
	}
}

func TestManifest_Validate(t *testing.T) {
	tests := []struct {
		name     string
		manifest *Manifest
		assert   func(t *testing.T, err error)
	}{
		{
			name:     "valid",
			manifest: manifestSample,
			assert: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "missing name",
			manifest: patchedManifestSample(func(mf *Manifest) {
				mf.Name = ""
			}),
			assert: func(t *testing.T, err error) {
				assert.EqualError(t, err, invalidManifestErr("missing name").Error())
			},
		},
		{
			name: "missing source code path",
			manifest: patchedManifestSample(func(mf *Manifest) {
				mf.SourceCodePath = ""
			}),
			assert: func(t *testing.T, err error) {
				assert.EqualError(t, err, invalidManifestErr("missing source code path").Error())
			},
		},
		{
			name: "missing action",
			manifest: patchedManifestSample(func(mf *Manifest) {
				mf.Action = ""
			}),
			assert: func(t *testing.T, err error) {
				assert.EqualError(t, err, invalidManifestErr("missing action").Error())
			},
		},
		{
			name: "invalid action",
			manifest: patchedManifestSample(func(mf *Manifest) {
				mf.Action = "HACK_ME"
			}),
			assert: func(t *testing.T, err error) {
				assert.EqualError(t, err, invalidManifestErr(fmt.Sprintf("unknown action 'HACK_ME' expecting one of %v", strings.Split(ActionNames(), "|"))).Error())
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assert(t, tt.manifest.Validate())
		})
	}
}

func TestManifest_DecryptSecrets(t *testing.T) {
	tests := []struct {
		name            string
		encryptSecrets  Secrets
		verbatimSecrets Secrets
		assert          func(t *testing.T, mf *Manifest, err error)
	}{
		{
			name: "ok",
			encryptSecrets: Secrets{
				"s1": "v1",
				"s2": "v2",
			},
			assert: func(t *testing.T, mf *Manifest, err error) {
				require.NoError(t, err)
				assert.Equal(t, Secrets{
					"s1": "v1",
					"s2": "v2",
				}, mf.Secrets)
			},
		},
		{
			name: "with cleartext secrets",
			verbatimSecrets: Secrets{
				"s1": "v1",
			},
			assert: func(t *testing.T, mf *Manifest, err error) {
				assert.EqualError(t, err, "cannot decrypt secret 's1', please check the manifest")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := os.Setenv(EnvKeySecretsPassword, "P@ssw0rd!")
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = os.Unsetenv(EnvKeySecretsPassword)
			})

			mf := patchedManifestSample(func(mf *Manifest) {
				mf.Secrets = Secrets{}

				var err error
				for key, val := range tt.encryptSecrets {
					mf.Secrets[key], err = EncryptSecret("P@ssw0rd!", val)
					require.NoError(t, err)
				}

				for key, val := range tt.verbatimSecrets {
					mf.Secrets[key] = val
				}
			})

			tt.assert(t, mf, mf.DecryptSecrets())
		})
	}
}

func patchedManifestSample(patch func(mf *Manifest)) *Manifest {
	patched := *manifestSample
	patch(&patched)
	return &patched
}
