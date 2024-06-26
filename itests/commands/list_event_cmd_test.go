//go:build itest

package commands

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jfrog/jfrog-cli-platform-services/itests/infra"
)

type listEventTestCase struct {
	name         string
	only         bool
	skip         bool
	wantErr      error
	commandArgs  []string
	assertOutput func(t require.TestingT, output []byte)
}

func TestListEventCommand(t *testing.T) {
	infra.RunITests([]infra.TestDefinition{
		listEventTestSpec(listEventTestCase{
			name: "list-event",
			assertOutput: func(t require.TestingT, content []byte) {
				var events []string
				require.NoError(t, json.Unmarshal(content, &events))
				assert.Truef(t, len(events) > 0, "no events received")
			},
		}),
	}, t)
}

func listEventTestSpec(tc listEventTestCase) infra.TestDefinition {
	return infra.TestDefinition{
		Name:          tc.name,
		Only:          tc.only,
		Skip:          tc.skip,
		CaptureOutput: true,
		Test: func(it *infra.Test) {
			cmd := append([]string{"worker", "list-event"}, tc.commandArgs...)

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
