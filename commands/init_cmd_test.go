package commands

import (
	"fmt"
	"os"
	"path"
	"regexp"
	"strings"
	"testing"

	"github.com/jfrog/jfrog-cli-platform-services/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runCommandFunc func(args ...string) error

func TestGetCommand(t *testing.T) {
	cmd := GetInitCommand()

	assert.Equalf(t, "init", cmd.Name, "Invalid command name")
	assert.NotEmptyf(t, cmd.Description, "No description")

	require.Lenf(t, cmd.Aliases, 1, "No alias")
	assert.Equal(t, "i", cmd.Aliases[0], "Invalid alias")

	require.Lenf(t, cmd.Arguments, 2, "Invalid number of argument provided")
	assert.Equalf(t, "action", cmd.Arguments[0].Name, "Invalid first argument")
	assert.NotEmptyf(t, cmd.Arguments[0].Description, "Action argument should be described")
	assert.Equalf(t, "worker-name", cmd.Arguments[1].Name, "Invalid second argument")
	assert.NotEmptyf(t, cmd.Arguments[1].Description, "Name argument should be described")

	assert.NotNilf(t, cmd.Action, "An action should be provided")
}

func TestInitWorker(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T, runCommand runCommandFunc)
	}{
		{
			name: "missing action and name",
			test: func(t *testing.T, runCommand runCommandFunc) {
				err := runCommand("worker", "init")
				assert.EqualError(t, err, "the action or worker name is missing, please see 'jf worker init --help'")
			},
		},
		{
			name: "missing name",
			test: func(t *testing.T, runCommand runCommandFunc) {
				err := runCommand("worker", "init", "BEFORE_DOWNLOAD")
				assert.EqualError(t, err, "the action or worker name is missing, please see 'jf worker init --help'")
			},
		},
		{
			name: "invalid action",
			test: func(t *testing.T, runCommand runCommandFunc) {
				err := runCommand("worker", "init", "HACK_SYSTEM", "root")
				assert.EqualError(t, err, fmt.Sprintf("invalid action '%s' action should be one of: %s", "HACK_SYSTEM", strings.Split(model.ActionNames(), "|")))
			},
		},
		{
			name: "generate",
			test: testGenerateAllActions,
		},
		{
			name: "overwrite manifest with force",
			test: testGenerateWithOverwrite("manifest.json", true),
		},
		{
			name: "dont overwrite manifest without force",
			test: testGenerateWithOverwrite("manifest.json", false),
		},
		{
			name: "overwrite sourceCode with force",
			test: testGenerateWithOverwrite("worker.ts", true),
		},
		{
			name: "dont overwrite sourceCode without force",
			test: testGenerateWithOverwrite("worker.ts", false),
		},
		{
			name: "overwrite testSourceCode with force",
			test: testGenerateWithOverwrite("worker.spec.ts", true),
		},
		{
			name: "dont overwrite testSourceCode without force",
			test: testGenerateWithOverwrite("worker.spec.ts", false),
		},
		{
			name: "overwrite package.json with force",
			test: testGenerateWithOverwrite("package.json", true),
		},
		{
			name: "dont overwrite package.json without force",
			test: testGenerateWithOverwrite("package.json", false),
		},
		{
			name: "overwrite tsconfig.json with force",
			test: testGenerateWithOverwrite("tsconfig.json", true),
		},
		{
			name: "dont overwrite tsconfig.json without force",
			test: testGenerateWithOverwrite("tsconfig.json", false),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.test(t, createCliRunner(t, GetInitCommand()))
		})
	}
}

func testGenerateWithOverwrite(fileName string, overwrite bool) func(t *testing.T, runCommand runCommandFunc) {
	return func(t *testing.T, runCommand runCommandFunc) {
		dir, err := os.MkdirTemp("", "worker-*.init")
		require.NoError(t, err)

		t.Cleanup(func() {
			_ = os.RemoveAll(dir)
		})

		// Simulate an existing file
		f, err := os.OpenFile(path.Join(dir, fileName), os.O_CREATE|os.O_WRONLY, os.ModePerm)
		require.NoError(t, err)
		_, err = f.WriteString("dummy content")
		require.NoError(t, err)
		require.NoError(t, f.Close())

		oldPwd, err := os.Getwd()
		require.NoError(t, err)

		err = os.Chdir(dir)
		require.NoError(t, err)

		t.Cleanup(func() {
			require.NoError(t, os.Chdir(oldPwd))
		})

		workerName := path.Base(dir)

		cmd := []string{"worker", "init"}
		if overwrite {
			cmd = append(cmd, "--force")
		}
		cmd = append(cmd, "BEFORE_DOWNLOAD", workerName)

		err = runCommand(cmd...)

		if overwrite {
			assert.NoError(t, err)
		} else {
			require.NotNilf(t, err, "an error was expected")
			errMatched, err := regexp.MatchString(fmt.Sprintf(`%s already exists in \S+/%s, please use '--force' to overwrite if you know what you are doing`, fileName, workerName), err.Error())
			require.NoError(t, err)
			assert.True(t, errMatched)
		}
	}
}

func testGenerateAction(actionName string, withTests bool, runCommand runCommandFunc) func(t *testing.T) {
	return func(t *testing.T) {
		dir, workerName := prepareWorkerDirForTest(t)

		manifestPath := path.Join(dir, "manifest.json")
		workerSourcePath := path.Join(dir, "worker.ts")
		workerTestSourcePath := path.Join(dir, "worker.spec.ts")
		packageJsonPath := path.Join(dir, "package.json")
		tsconfigJsonPath := path.Join(dir, "tsconfig.json")

		wantManifest := generateForTest(t, actionName, workerName, "manifest.json_template", !withTests)
		wantPackageJson := generateForTest(t, actionName, workerName, "package.json_template", !withTests)
		wantWorkerSource := generateForTest(t, actionName, workerName, actionName+".ts_template", !withTests)
		wantWorkerTestSource := generateForTest(t, actionName, workerName, actionName+".spec.ts_template", !withTests)
		wantTsconfig := generateForTest(t, actionName, workerName, "tsconfig.json_template", !withTests)

		commandArgs := []string{"worker", "init"}
		if !withTests {
			commandArgs = append(commandArgs, "--"+model.FlagNoTest)
		}
		commandArgs = append(commandArgs, actionName, workerName)

		err := runCommand(commandArgs...)
		require.NoError(t, err)

		gotManifest, err := os.ReadFile(manifestPath)
		require.NoErrorf(t, err, "Cannot get manifest content")
		assert.Equalf(t, wantManifest, string(gotManifest), "Invalid manifest content")

		gotSource, err := os.ReadFile(workerSourcePath)
		require.NoErrorf(t, err, "Cannot get worker source code")
		assert.Equalf(t, wantWorkerSource, string(gotSource), "Invalid worker source code")

		if withTests {
			gotTestSource, err := os.ReadFile(workerTestSourcePath)
			require.NoErrorf(t, err, "Cannot get worker test source code")
			assert.Equalf(t, wantWorkerTestSource, string(gotTestSource), "Invalid worker test source code")
		}

		gotPackageJson, err := os.ReadFile(packageJsonPath)
		require.NoErrorf(t, err, "Cannot get worker package.json")
		assert.Equalf(t, wantPackageJson, string(gotPackageJson), "Invalid worker package.json")

		gotTsconfigJson, err := os.ReadFile(tsconfigJsonPath)
		require.NoErrorf(t, err, "Cannot get worker tsconfig.json")
		assert.Equalf(t, wantTsconfig, string(gotTsconfigJson), "Invalid worker tsconfig.json")
	}
}

func testGenerateAllActions(t *testing.T, runCommand runCommandFunc) {
	for _, actionName := range strings.Split(model.ActionNames(), "|") {
		t.Run(actionName, testGenerateAction(actionName, true, runCommand))
		t.Run(actionName+" without tests", testGenerateAction(actionName, false, runCommand))
	}
}
