//go:build test
// +build test

package common

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jfrog/jfrog-cli-platform-services/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var manifestSample = &model.Manifest{
	Name:           "my-worker",
	Description:    "my worker description",
	SourceCodePath: "./my-worker.ts",
	Action:         "BEFORE_DOWNLOAD",
	Enabled:        true,
	Debug:          true,
	ProjectKey:     "a-project",
	Secrets: model.Secrets{
		"hidden1": "hidden1.value",
		"hidden2": "hidden2.value",
	},
	FilterCriteria: &model.FilterCriteria{
		ArtifactFilterCriteria: &model.ArtifactFilterCriteria{
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
		sample   *model.Manifest
		assert   func(t *testing.T, mf *model.Manifest, readErr error)
	}{
		{
			name:   "in current dir",
			sample: manifestSample,
			assert: func(t *testing.T, mf *model.Manifest, readErr error) {
				require.NoError(t, readErr)
				assert.Equal(t, manifestSample, mf)
			},
		},
		{
			name:     "with dir as argument",
			sample:   manifestSample,
			dirAsArg: true,
			assert: func(t *testing.T, mf *model.Manifest, readErr error) {
				require.NoError(t, readErr)
				assert.Equal(t, manifestSample, mf)
			},
		},
		{
			name: "with missing manifest",
			assert: func(t *testing.T, mf *model.Manifest, readErr error) {
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

func TestReadManifestPreservesOmittedAndEmptyProperties(t *testing.T) {
	tests := []struct {
		name       string
		properties string
		assert     func(*testing.T, map[string]string)
	}{
		{
			name: "omitted",
			assert: func(t *testing.T, properties map[string]string) {
				assert.Nil(t, properties)
			},
		},
		{
			name:       "empty",
			properties: `,"properties":{}`,
			assert: func(t *testing.T, properties map[string]string) {
				assert.NotNil(t, properties)
				assert.Empty(t, properties)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			manifest := `{"name":"worker","sourceCodePath":"worker.ts","action":"GENERIC_EVENT"` + tt.properties + `}`
			require.NoError(t, os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o600))

			got, err := ReadManifest(dir)
			require.NoError(t, err)
			tt.assert(t, got.Properties)
		})
	}
}

func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
}

func rejected(t assert.TestingT, err error, _ ...interface{}) bool {
	assert.ErrorContains(t, err, "must be a relative path inside the worker directory")
	return false
}

func TestManifest_ReadSourceCode(t *testing.T) {
	tests := []struct {
		name       string
		sourceCode string
		manifest   *model.Manifest
		// setup optionally prepares the folder and returns the manifest to use instead of manifest.
		// outside is an existing file located outside dir.
		setup   func(t *testing.T, dir, outside string) *model.Manifest
		want    string
		wantErr assert.ErrorAssertionFunc
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
		{
			name: "nested relative path",
			setup: func(t *testing.T, dir, _ string) *model.Manifest {
				require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o700))
				require.NoError(t, os.WriteFile(filepath.Join(dir, "my-worker.ts"), []byte("nested"), 0o600))
				return patchedManifestSample(func(mf *model.Manifest) { mf.SourceCodePath = "sub/../my-worker.ts" })
			},
			want:    "nested",
			wantErr: assert.NoError,
		},
		{
			name: "symlink inside the directory",
			setup: func(t *testing.T, dir, _ string) *model.Manifest {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "my-worker.ts"), []byte("linked"), 0o600))
				symlinkOrSkip(t, "my-worker.ts", filepath.Join(dir, "link.ts"))
				return patchedManifestSample(func(mf *model.Manifest) { mf.SourceCodePath = "link.ts" })
			},
			want:    "linked",
			wantErr: assert.NoError,
		},
		{
			name: "absolute path outside the directory rejected",
			setup: func(t *testing.T, _, outside string) *model.Manifest {
				return patchedManifestSample(func(mf *model.Manifest) { mf.SourceCodePath = outside })
			},
			wantErr: rejected,
		},
		{
			name: "absolute path inside the directory rejected",
			setup: func(t *testing.T, dir, _ string) *model.Manifest {
				require.NoError(t, os.WriteFile(filepath.Join(dir, "my-worker.ts"), []byte("x"), 0o600))
				return patchedManifestSample(func(mf *model.Manifest) { mf.SourceCodePath = filepath.Join(dir, "my-worker.ts") })
			},
			wantErr: rejected,
		},
		{
			name: "parent traversal rejected",
			setup: func(t *testing.T, dir, outside string) *model.Manifest {
				rel, err := filepath.Rel(dir, outside)
				require.NoError(t, err)
				return patchedManifestSample(func(mf *model.Manifest) { mf.SourceCodePath = rel })
			},
			wantErr: rejected,
		},
		{
			name: "symlink escaping the directory rejected",
			setup: func(t *testing.T, dir, outside string) *model.Manifest {
				symlinkOrSkip(t, outside, filepath.Join(dir, "escape.ts"))
				return patchedManifestSample(func(mf *model.Manifest) { mf.SourceCodePath = "escape.ts" })
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				assert.ErrorContains(t, err, "invalid manifest:")
				assert.ErrorContains(t, err, "cannot read sourceCodePath 'escape.ts'")
				assert.ErrorContains(t, err, "path escapes")
				assert.False(t, os.IsNotExist(err))
				return false
			},
		},
		{
			name: "directory instead of a file",
			setup: func(t *testing.T, dir, _ string) *model.Manifest {
				require.NoError(t, os.Mkdir(filepath.Join(dir, "sub"), 0o700))
				return patchedManifestSample(func(mf *model.Manifest) { mf.SourceCodePath = "sub" })
			},
			wantErr: func(t assert.TestingT, err error, i ...interface{}) bool {
				assert.ErrorContains(t, err, "invalid manifest: cannot read sourceCodePath 'sub'")
				assert.NotContains(t, err.Error(), "path escapes")
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

			manifest := tt.manifest
			if tt.setup != nil {
				outsideFolder := t.TempDir()
				outside := filepath.Join(outsideFolder, "secret.txt")
				require.NoError(t, os.WriteFile(outside, []byte("secret"), 0o600))
				manifest = tt.setup(t, manifestFolder, outside)
			}

			t.Chdir(manifestFolder)

			got, err := ReadSourceCode(manifest)
			if !tt.wantErr(t, err, "ReadSourceCode()") {
				return
			}

			assert.Equalf(t, tt.want, got, "ReadSourceCode()")
		})
	}
}

func TestManifest_Validate(t *testing.T) {
	sampleActions := LoadSampleActions(t)

	tests := []struct {
		name        string
		manifest    *model.Manifest
		assert      func(t *testing.T, err error)
		actionsMeta ActionsMetadata
	}{
		{
			name:        "valid",
			manifest:    manifestSample,
			actionsMeta: sampleActions,
			assert: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name:        "missing name",
			actionsMeta: sampleActions,
			manifest: patchedManifestSample(func(mf *model.Manifest) {
				mf.Name = ""
			}),
			assert: func(t *testing.T, err error) {
				assert.EqualError(t, err, invalidManifestErr("missing name").Error())
			},
		},
		{
			name:        "invalid name",
			actionsMeta: sampleActions,
			manifest: patchedManifestSample(func(mf *model.Manifest) {
				mf.Name = "../x?y"
			}),
			assert: func(t *testing.T, err error) {
				assert.EqualError(t, err, invalidManifestErr("invalid worker key '../x?y': must start with a letter").Error())
			},
		},
		{
			name:        "name too long",
			actionsMeta: sampleActions,
			manifest: patchedManifestSample(func(mf *model.Manifest) {
				mf.Name = "a" + strings.Repeat("b", 40)
			}),
			assert: func(t *testing.T, err error) {
				assert.ErrorContains(t, err, "must be lower than 40 chars long")
			},
		},
		{
			name:        "missing source code path",
			actionsMeta: sampleActions,
			manifest: patchedManifestSample(func(mf *model.Manifest) {
				mf.SourceCodePath = ""
			}),
			assert: func(t *testing.T, err error) {
				assert.EqualError(t, err, invalidManifestErr("missing source code path").Error())
			},
		},
		{
			name:        "missing action",
			actionsMeta: sampleActions,
			manifest: patchedManifestSample(func(mf *model.Manifest) {
				mf.Action = ""
			}),
			assert: func(t *testing.T, err error) {
				assert.EqualError(t, err, invalidManifestErr("missing action").Error())
			},
		},
		{
			name:        "invalid action",
			actionsMeta: sampleActions,
			manifest: patchedManifestSample(func(mf *model.Manifest) {
				mf.Action = "HACK_ME"
			}),
			assert: func(t *testing.T, err error) {
				assert.Regexp(t, regexp.MustCompile("action 'HACK_ME' not found"), err)
			},
		},
		{
			name: "no action validation if no actions metadata",
			manifest: patchedManifestSample(func(mf *model.Manifest) {
				mf.Action = "HACK_ME"
			}),
			assert: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assert(t, ValidateManifest(tt.manifest, tt.actionsMeta))
		})
	}
}

func TestManifest_DecryptSecrets(t *testing.T) {
	tests := []struct {
		name            string
		encryptSecrets  model.Secrets
		verbatimSecrets model.Secrets
		assert          func(t *testing.T, mf *model.Manifest, err error)
	}{
		{
			name: "ok",
			encryptSecrets: model.Secrets{
				"s1": "v1",
				"s2": "v2",
			},
			assert: func(t *testing.T, mf *model.Manifest, err error) {
				require.NoError(t, err)
				assert.Equal(t, model.Secrets{
					"s1": "v1",
					"s2": "v2",
				}, mf.Secrets)
			},
		},
		{
			name: "with cleartext secrets",
			verbatimSecrets: model.Secrets{
				"s1": "v1",
			},
			assert: func(t *testing.T, mf *model.Manifest, err error) {
				assert.EqualError(t, err, "cannot decrypt secret 's1', please check the manifest")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := os.Setenv(model.EnvKeySecretsPassword, "P@ssw0rd!")
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = os.Unsetenv(model.EnvKeySecretsPassword)
			})

			mf := patchedManifestSample(func(mf *model.Manifest) {
				mf.Secrets = model.Secrets{}

				var err error
				for key, val := range tt.encryptSecrets {
					mf.Secrets[key], err = EncryptSecret("P@ssw0rd!", val)
					require.NoError(t, err)
				}

				for key, val := range tt.verbatimSecrets {
					mf.Secrets[key] = val
				}
			})

			tt.assert(t, mf, DecryptManifestSecrets(mf))
		})
	}
}

func TestManifest_ValidateFilterCriteria(t *testing.T) {
	tests := []struct {
		name       string
		criteria   *model.FilterCriteria
		actionMeta *model.ActionMetadata
		wantErr    error
	}{
		{
			name: "no mandatory filters",
			actionMeta: &model.ActionMetadata{
				FilterType:      model.FilterTypeSchedule,
				MandatoryFilter: false,
			},
			criteria: &model.FilterCriteria{
				Schedule:               &model.ScheduleFilterCriteria{},
				ArtifactFilterCriteria: &model.ArtifactFilterCriteria{},
			},
		},
		{
			name: "valid schedule criteria",
			actionMeta: &model.ActionMetadata{
				FilterType:      model.FilterTypeSchedule,
				MandatoryFilter: true,
			},
			criteria: &model.FilterCriteria{
				Schedule: &model.ScheduleFilterCriteria{
					Cron:     "0 1 1 * *",
					Timezone: "UTC",
				},
			},
		},
		{
			name: "valid artifact filter criteria",
			actionMeta: &model.ActionMetadata{
				FilterType:      model.FilterTypeRepo,
				MandatoryFilter: true,
			},
			criteria: &model.FilterCriteria{
				ArtifactFilterCriteria: &model.ArtifactFilterCriteria{
					RepoKeys: []string{"my-repo-local"},
				},
			},
		},
		{
			name: "valid with any local",
			actionMeta: &model.ActionMetadata{
				FilterType:      model.FilterTypeRepo,
				MandatoryFilter: true,
			},
			criteria: &model.FilterCriteria{
				ArtifactFilterCriteria: &model.ArtifactFilterCriteria{
					AnyLocal: true,
				},
			},
		},
		{
			name: "valid with any federated",
			actionMeta: &model.ActionMetadata{
				FilterType:      model.FilterTypeRepo,
				MandatoryFilter: true,
			},
			criteria: &model.FilterCriteria{
				ArtifactFilterCriteria: &model.ArtifactFilterCriteria{
					AnyFederated: true,
				},
			},
		},
		{
			name: "valid with any local",
			actionMeta: &model.ActionMetadata{
				FilterType:      model.FilterTypeRepo,
				MandatoryFilter: true,
			},
			criteria: &model.FilterCriteria{
				ArtifactFilterCriteria: &model.ArtifactFilterCriteria{
					AnyRemote: true,
				},
			},
		},
		{
			name: "only schedule for scheduled action",
			actionMeta: &model.ActionMetadata{
				FilterType:      model.FilterTypeSchedule,
				MandatoryFilter: true,
			},
			criteria: &model.FilterCriteria{
				Schedule:               &model.ScheduleFilterCriteria{},
				ArtifactFilterCriteria: &model.ArtifactFilterCriteria{},
			},
			wantErr: invalidManifestErr("scheduled event cannot have artifact filter criteria"),
		},
		{
			name: "only artifactFilterCriteria for non-scheduled action",
			actionMeta: &model.ActionMetadata{
				FilterType:      model.FilterTypeRepo,
				MandatoryFilter: true,
			},
			criteria: &model.FilterCriteria{
				Schedule:               &model.ScheduleFilterCriteria{},
				ArtifactFilterCriteria: &model.ArtifactFilterCriteria{},
			},
			wantErr: invalidManifestErr("repository events cannot have schedule criteria"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilterCriteria(tt.criteria, tt.actionMeta)
			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr.Error())
			}
		})
	}
}

func TestManifest_ValidateScheduleCriteria(t *testing.T) {
	tests := []struct {
		name     string
		criteria *model.ScheduleFilterCriteria
		wantErr  error
	}{
		{
			name: "valid",
			criteria: &model.ScheduleFilterCriteria{
				Cron:     "0 1 1 * *",
				Timezone: "UTC",
			},
		},
		{
			name: "missing cron",
			criteria: &model.ScheduleFilterCriteria{
				Timezone: "UTC",
			},
			wantErr: errors.New("missing cron expression"),
		},
		{
			name: "invalid cron",
			criteria: &model.ScheduleFilterCriteria{
				Cron:     "0 0 0 * * * *",
				Timezone: "UTC",
			},
			wantErr: errors.New("invalid cron expression"),
		},
		{
			name: "invalid timezone",
			criteria: &model.ScheduleFilterCriteria{
				Cron:     "0 1 1 * *",
				Timezone: "America/Toulouse",
			},
			wantErr: errors.New("invalid timezone 'America/Toulouse'"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateScheduleCriteria(tt.criteria)
			if tt.wantErr == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr.Error())
			}
		})
	}
}

func patchedManifestSample(patch func(mf *model.Manifest)) *model.Manifest {
	patched := *manifestSample
	patch(&patched)
	return &patched
}

func TestValidateWorkerKey(t *testing.T) {
	const (
		minMsg   = "must be at least 2 chars long"
		maxMsg   = "must be lower than 40 chars long"
		startMsg = "must start with a letter"
	)

	tests := []struct {
		key     string
		wantErr string // empty means valid, otherwise the full expected error
	}{
		{key: "ab"},
		{key: "a" + strings.Repeat("b", 39)},
		{key: "my-worker"},
		{key: "wk_1"},
		{key: "A-b_C-9"},
		{key: `a\b`},
		{key: "worker-1234567890-init"},
		{key: "a", wantErr: "invalid worker key 'a': " + minMsg},
		{key: "", wantErr: "invalid worker key '': " + minMsg},
		{key: "a" + strings.Repeat("b", 40), wantErr: "invalid worker key '" + "a" + strings.Repeat("b", 40) + "': " + maxMsg},
		{key: "1worker", wantErr: "invalid worker key '1worker': " + startMsg},
		{key: "-worker", wantErr: "invalid worker key '-worker': " + startMsg},
		{key: "_w", wantErr: "invalid worker key '_w': " + startMsg},
		{key: "..", wantErr: "invalid worker key '..': " + startMsg},
		{key: "../x?y", wantErr: "invalid worker key '../x?y': " + startMsg},
		{key: "ab/cd", wantErr: "invalid worker key 'ab/cd'"},
		{key: "x?y", wantErr: "invalid worker key 'x?y'"},
		{key: "a#b", wantErr: "invalid worker key 'a#b'"},
		{key: "a.b", wantErr: "invalid worker key 'a.b'"},
		{key: "my worker", wantErr: "invalid worker key 'my worker'"},
		{key: "a%2F", wantErr: "invalid worker key 'a%2F'"},
		{key: "wé", wantErr: "invalid worker key 'wé'"},
		{key: "ab\n", wantErr: "invalid worker key 'ab\n'"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			err := ValidateWorkerKey(tt.key)
			if tt.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.wantErr)
			}
		})
	}
}
