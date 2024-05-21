//go:build itest

package commands

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-platform-services/itests/infra"
	"github.com/jfrog/jfrog-cli-platform-services/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type listTestCase struct {
	name         string
	only         bool
	skip         bool
	wantErr      error
	commandArgs  []string
	initWorkers  []*model.WorkerDetails
	assertOutput func(t require.TestingT, output []byte)
}

func TestListCommand(t *testing.T) {
	initialWorkers := []*model.WorkerDetails{
		{
			Key:         fmt.Sprintf("w%v", time.Now().Unix()),
			Description: "My worker 0",
			Enabled:     true,
			SourceCode:  `export default async function() { return { "status": "OK" } }`,
			Action:      model.ActionGenericEvent,
		},
		{
			Key:         fmt.Sprintf("w%v", time.Now().Unix()+1),
			Description: "My worker 1",
			Enabled:     true,
			SourceCode:  `export default async function() { return { "status": "OK" } }`,
			Action:      model.ActionGenericEvent,
		},
		{
			Key:         fmt.Sprintf("w%v", time.Now().Unix()+2),
			Description: "My worker 2",
			Enabled:     true,
			SourceCode:  `export default async function() { return { "status": "OK" } }`,
			Action:      model.ActionBeforeDownload,
			FilterCriteria: model.FilterCriteria{
				ArtifactFilterCriteria: model.ArtifactFilterCriteria{
					RepoKeys: []string{"example-repo-local"},
				},
			},
		},
	}

	infra.RunITests([]infra.TestDefinition{
		listTestSpec(listTestCase{
			name:         "list",
			initWorkers:  initialWorkers,
			assertOutput: assertWorkerListCsv(initialWorkers),
		}),
		listTestSpec(listTestCase{
			name:         "list worker of type",
			commandArgs:  []string{"--" + model.FlagJsonOutput},
			initWorkers:  initialWorkers,
			assertOutput: assertWorkerListJSON(initialWorkers...),
		}),
		listTestSpec(listTestCase{
			name:         "list for JSON",
			commandArgs:  []string{"--" + model.FlagJsonOutput, "BEFORE_DOWNLOAD"},
			initWorkers:  initialWorkers,
			assertOutput: assertWorkerListJSON(initialWorkers[2:]...),
		}),
		listTestSpec(listTestCase{
			name:        "fails if invalid timeout",
			commandArgs: []string{"--" + model.FlagTimeout, "abc"},
			wantErr:     errors.New("invalid timeout provided"),
		}),
	}, t)
}

func listTestSpec(tc listTestCase) infra.TestDefinition {
	return infra.TestDefinition{
		Name:          tc.name,
		Only:          tc.only,
		Skip:          tc.skip,
		CaptureOutput: true,
		Test: func(it *infra.Test) {
			it.DeleteAllWorkers()

			it.Cleanup(func() {
				for _, initialWorker := range tc.initWorkers {
					it.DeleteWorker(initialWorker.Key)
				}
			})

			for _, initialWorker := range tc.initWorkers {
				it.CreateWorker(initialWorker)
			}

			cmd := append([]string{"worker", "list"}, tc.commandArgs...)

			err := it.RunCommand(cmd...)

			if tc.wantErr == nil {
				require.NoError(it, err)
				if tc.assertOutput != nil {
					tc.assertOutput(it, it.CapturedOutput())
				}
			} else {
				assert.EqualError(it, err, tc.wantErr.Error())
			}
		},
	}
}

func assertWorkerListCsv(workers []*model.WorkerDetails) func(t require.TestingT, content []byte) {
	var csvRecords [][]string

	for _, wk := range workers {
		csvRecords = append(csvRecords, []string{wk.Key, wk.Action, wk.Description, fmt.Sprint(wk.Enabled)})
	}

	return func(t require.TestingT, content []byte) {
		var wantCsv bytes.Buffer

		csvWriter := csv.NewWriter(&wantCsv)

		require.NoError(t, csvWriter.WriteAll(csvRecords))

		assert.Equal(t, string(wantCsv.Bytes()), string(content))
	}
}

func assertWorkerListJSON(workers ...*model.WorkerDetails) func(t require.TestingT, content []byte) {
	var csvRecords [][]string

	for _, wk := range workers {
		csvRecords = append(csvRecords, []string{wk.Key, wk.Action, wk.Description, fmt.Sprint(wk.Enabled)})
	}

	return func(t require.TestingT, content []byte) {
		gotWorkers := struct {
			Workers []*model.WorkerDetails `json:"workers"`
		}{}

		require.NoError(t, json.Unmarshal(content, &gotWorkers))

		assert.Equalf(t, len(workers), len(gotWorkers.Workers), "Length mismatch")

		for i, wantWorker := range workers {
			gotWorker := gotWorkers.Workers[i]

			assert.Equalf(t, wantWorker.Key, gotWorker.Key, "Key mismatch")
			assert.Equalf(t, wantWorker.Action, gotWorker.Action, "Action mismatch")
			assert.Equalf(t, wantWorker.Description, gotWorker.Description, "Description mismatch")
			assert.Equalf(t, wantWorker.Enabled, gotWorker.Enabled, "Enabled mismatch")
		}
	}
}
