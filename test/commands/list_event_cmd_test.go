//go:build itest

package commands

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jfrog/jfrog-cli-platform-services/test/infra"
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
				events := strings.Split(string(content), ", ")
				assert.Truef(t, len(events) > 2, "no events received")
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
			cmd := append([]string{infra.AppName, "list-event"}, tc.commandArgs...)

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
