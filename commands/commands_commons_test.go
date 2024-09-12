package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"path"
	"testing"
	"text/template"

	"github.com/google/uuid"

	"github.com/stretchr/testify/assert"

	"github.com/jfrog/jfrog-cli-core/v2/plugins"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"

	"github.com/stretchr/testify/require"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

const secretPassword = "P@ssw0rd!"

func Test_cleanImports(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "case 1",
			source: `import { a } from 'b'; export default async (context: a) => ({ status: 200 })`,
			want:   "export default async (context: a) => ({ status: 200 })",
		},
		{
			name: "case 2",
			source: `
				import { a } from 'b'; 
				import { c, d } from 'e';

				export default async (context: a) => ({ status: 200 })`,
			want: "export default async (context: a) => ({ status: 200 })",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := model.CleanImports(tt.source)
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_extractProjectAndKeyFromCommandContext(t *testing.T) {
	tests := []struct {
		name         string
		c            stringFlagAware
		args         []string
		minArguments int
		onlyGeneric  bool
		eventType    string
		assert       func(t *testing.T, manifestWorkerKey, manifestProjectKey, extractedWorkerKey, extractedProjectKey string, err error)
	}{
		{
			name:         "project and worker key from args",
			c:            mockStringFlagAware{map[string]string{model.FlagProjectKey: "proj1"}},
			args:         []string{"worker1"},
			minArguments: 0,
			onlyGeneric:  false,
			assert: func(t *testing.T, _, _, extractedWorkerKey, extractedProjectKey string, err error) {
				require.NoError(t, err)
				assert.Equal(t, "worker1", extractedWorkerKey)
				assert.Equal(t, "proj1", extractedProjectKey)
			},
		},
		{
			name:         "project and worker key from manifest",
			c:            mockStringFlagAware{map[string]string{}},
			args:         []string{},
			minArguments: 0,
			onlyGeneric:  false,
			assert: func(t *testing.T, manifestWorkerKey, manifestProjectKey, extractedWorkerKey, extractedProjectKey string, err error) {
				require.NoError(t, err)
				assert.Equal(t, manifestWorkerKey, extractedWorkerKey)
				assert.Equal(t, manifestProjectKey, extractedProjectKey)
			},
		},
		{
			name:         "only generic event allowed",
			c:            mockStringFlagAware{map[string]string{model.FlagProjectKey: ""}},
			args:         []string{},
			minArguments: 0,
			onlyGeneric:  true,
			eventType:    "BEFORE_DOWNLOAD",
			assert: func(t *testing.T, _, _, _, _ string, err error) {
				assert.EqualError(t, err, "only the GENERIC_EVENT actions are executable. Got BEFORE_DOWNLOAD")
			},
		},
		{
			name:         "min arguments count not satisfied",
			c:            mockStringFlagAware{map[string]string{model.FlagProjectKey: ""}},
			args:         []string{"@jsonPayload.json"},
			minArguments: 1,
			onlyGeneric:  false,
			assert: func(t *testing.T, manifestWorkerKey, manifestProjectKey, extractedWorkerKey, extractedProjectKey string, err error) {
				require.NoError(t, err)
				assert.Equal(t, manifestWorkerKey, extractedWorkerKey)
				assert.Equal(t, manifestProjectKey, extractedProjectKey)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, manifestWorkerKey := prepareWorkerDirForTest(t)
			manifestProjectKey := uuid.NewString()

			mf := model.Manifest{
				Name:           manifestWorkerKey,
				ProjectKey:     manifestProjectKey,
				Action:         tt.eventType,
				SourceCodePath: "./worker.ts",
			}

			if mf.Action == "" {
				mf.Action = "GENERIC_EVENT"
			}

			require.NoError(t, mf.Save(dir))

			workerKey, projectKey, err := extractProjectAndKeyFromCommandContext(tt.c, tt.args, tt.minArguments, tt.onlyGeneric)

			tt.assert(t, manifestWorkerKey, manifestProjectKey, workerKey, projectKey, err)
		})
	}
}

type mockStringFlagAware struct {
	data map[string]string
}

func (m mockStringFlagAware) GetStringFlagValue(flag string) string {
	return m.data[flag]
}

func prepareWorkerDirForTest(t *testing.T) (string, string) {
	dir, err := os.MkdirTemp("", "worker-*-init")
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	oldPwd, err := os.Getwd()
	require.NoError(t, err)

	err = os.Chdir(dir)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, os.Chdir(oldPwd))
	})

	workerName := path.Base(dir)

	return dir, workerName
}

func generateForTest(t require.TestingT, action string, workerName string, templateName string, skipTests ...bool) string {
	tpl, err := template.New(templateName).ParseFS(templates, "templates/"+templateName)
	require.NoErrorf(t, err, "cannot initialize the template for %s", action)

	var out bytes.Buffer
	err = tpl.Execute(&out, map[string]any{
		"Action":      action,
		"WorkerName":  workerName,
		"HasCriteria": model.ActionNeedsCriteria(action),
		"HasTests":    len(skipTests) == 0 || !skipTests[0],
	})
	require.NoError(t, err)

	return out.String()
}

func mustJsonMarshal(t *testing.T, data any) string {
	out, err := json.Marshal(data)
	require.NoError(t, err)
	return string(out)
}

func createTempFileWithContent(t *testing.T, content string) string {
	file, err := os.CreateTemp("", "wks-cli-*.test")
	require.NoError(t, err)

	t.Cleanup(func() {
		// We do not care about this error
		_ = os.Remove(file.Name())
	})

	_, err = file.Write([]byte(content))
	require.NoError(t, err)

	return file.Name()
}

func createCliRunner(t *testing.T, commands ...components.Command) func(args ...string) error {
	app := components.App{}
	app.Name = "worker"
	app.Commands = commands

	runCli := plugins.RunCliWithPlugin(app)

	return func(args ...string) error {
		oldArgs := os.Args
		t.Cleanup(func() {
			os.Args = oldArgs
		})
		os.Args = args
		return runCli()
	}
}

func patchManifest(t require.TestingT, applyPatch func(mf *model.Manifest), dir ...string) {
	mf, err := model.ReadManifest(dir...)
	require.NoError(t, err)

	applyPatch(mf)

	require.NoError(t, mf.Save(dir...))
}

func getActionSourceCode(t require.TestingT, actionName string) string {
	templateName := actionName + ".ts_template"
	content, err := templates.ReadFile("templates/" + templateName)
	require.NoError(t, err)
	return string(content)
}

func mustEncryptSecret(t require.TestingT, secretValue string, password ...string) string {
	key := secretPassword
	if len(password) > 0 {
		key = password[0]
	}
	encryptedValue, err := model.EncryptSecret(key, secretValue)
	require.NoError(t, err)
	return encryptedValue
}
