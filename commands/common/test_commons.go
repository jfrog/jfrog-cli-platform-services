//go:build test || itest

package common

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	"testing"
	"text/template"

	"github.com/jfrog/jfrog-cli-core/v2/plugins"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Test interface {
	require.TestingT
	Cleanup(func())
}

const SecretPassword = "P@ssw0rd!"

//go:embed testdata/actions/*
var sampleActions embed.FS

func SetCliIn(reader io.Reader) {
	cliIn = reader
}

func SetCliOut(writer io.Writer) {
	cliOut = writer
}

func PrepareWorkerDirForTest(t *testing.T) (string, string) {
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

func GenerateFromSamples(t require.TestingT, templates embed.FS, action string, workerName string, projectKey string, templateName string, skipTests ...bool) string {
	tpl, err := template.New(templateName).ParseFS(templates, "templates/"+templateName)
	require.NoErrorf(t, err, "cannot initialize the template for %s", action)

	actionsMeta := LoadSampleActions(t)

	actionMeta, err := actionsMeta.FindAction(action)
	require.NoError(t, err)

	params := map[string]any{
		"Action":                actionMeta.Action.Name,
		"Application":           actionMeta.Action.Application,
		"WorkerName":            workerName,
		"HasRepoFilterCriteria": actionMeta.MandatoryFilter && actionMeta.FilterType == model.FilterTypeRepo,
		"HasTests":              len(skipTests) == 0 || !skipTests[0],
		"HasRequestType":        actionMeta.ExecutionRequestType != "",
		"ExecutionRequestType":  actionMeta.ExecutionRequestType,
		"ProjectKey":            projectKey,
	}

	usedTypes := ExtractActionUsedTypes(actionMeta)
	if len(usedTypes) > 0 {
		params["UsedTypes"] = strings.Join(usedTypes, ", ")
	}

	var out bytes.Buffer

	err = tpl.Execute(&out, params)
	require.NoError(t, err)

	return out.String()
}

func MustJsonMarshal(t *testing.T, data any) string {
	out, err := json.Marshal(data)
	require.NoError(t, err)
	return string(out)
}

func CreateTempFileWithContent(t Test, content string) string {
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

func CreateCliRunner(t Test, commands ...components.Command) func(args ...string) error {
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

func PatchManifest(t require.TestingT, applyPatch func(mf *model.Manifest), dir ...string) {
	mf, err := ReadManifest(dir...)
	require.NoError(t, err)

	applyPatch(mf)

	require.NoError(t, SaveManifest(mf, dir...))
}

func MustEncryptSecret(t require.TestingT, secretValue string, password ...string) string {
	key := SecretPassword
	if len(password) > 0 {
		key = password[0]
	}
	encryptedValue, err := EncryptSecret(key, secretValue)
	require.NoError(t, err)
	return encryptedValue
}

func LoadSampleActions(t require.TestingT) ActionsMetadata {
	var metadata ActionsMetadata

	actionsFiles, err := sampleActions.ReadDir("testdata/actions")
	require.NoError(t, err)

	for _, file := range actionsFiles {
		content, err := sampleActions.ReadFile("testdata/actions/" + file.Name())
		require.NoError(t, err)

		action := &model.ActionMetadata{}
		err = json.Unmarshal(content, action)
		require.NoError(t, err)

		metadata = append(metadata, action)
	}

	return metadata
}

func LoadSampleActionEvents(t require.TestingT) []string {
	var events []string

	actionsMeta := LoadSampleActions(t)
	for _, md := range actionsMeta {
		events = append(events, md.Action.Name)
	}

	return events
}

func TestSetEnv(t Test, key, value string) {
	err := os.Setenv(key, value)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := os.Unsetenv(key); err != nil {
			log.Warn(fmt.Sprintf("cannot unset %s: %+v", key, err))
		}
	})
}

type AssertOutputFunc func(t *testing.T, stdOutput []byte, err error)

func AssertOutputErrorRegexp(pattern string) AssertOutputFunc {
	return func(t *testing.T, stdOutput []byte, err error) {
		require.Error(t, err)
		assert.Regexpf(t, pattern, err.Error(), "expected error to match pattern %q, got %+v", pattern, err)
	}
}

func AssertOutputError(errorMessage string, errorMessageArgs ...any) AssertOutputFunc {
	return func(t *testing.T, stdOutput []byte, err error) {
		require.Error(t, err)
		assert.EqualError(t, err, fmt.Sprintf(errorMessage, errorMessageArgs...))
	}
}

func AssertOutputJson[T any](wantResponse T) AssertOutputFunc {
	return func(t *testing.T, output []byte, err error) {
		require.NoError(t, err)

		outputData := new(T)

		err = json.Unmarshal(output, outputData)
		require.NoError(t, err)

		assert.Equal(t, wantResponse, *outputData)
	}
}

func AssertOutputText(wantResponse string, message string, args ...any) AssertOutputFunc {
	return func(t *testing.T, output []byte, err error) {
		require.NoError(t, err)
		assert.Equalf(t, strings.TrimSpace(wantResponse), strings.TrimSpace(string(output)), message, args...)
	}
}

type IntFlagMap map[string]int

func (m IntFlagMap) GetIntFlagValue(key string) (int, error) {
	val := m[key]
	return val, nil
}

func (m IntFlagMap) IsFlagSet(key string) bool {
	_, ok := m[key]
	return ok
}

func SortWorkers(workers []*model.WorkerDetails) {
	sort.Slice(workers, func(i, j int) bool {
		return workers[i].Key < workers[j].Key
	})
}
