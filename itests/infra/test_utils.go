//go:build itest

package infra

import (
	"encoding/json"
	"os"

	"github.com/jfrog/jfrog-cli-platform-services/model"

	"github.com/stretchr/testify/require"
)

type cliTestingT interface {
	require.TestingT
	Cleanup(func())
}

func MustJsonMarshal(t cliTestingT, data any) string {
	out, err := json.Marshal(data)
	require.NoError(t, err)
	return string(out)
}

func CreateTempFileWithContent(t cliTestingT, content string) string {
	file, err := os.CreateTemp("", "wks-cli-*.test")
	require.NoError(t, err)

	t.Cleanup(func() {
		// We do not care about an error here
		_ = os.Remove(file.Name())
	})

	_, err = file.Write([]byte(content))
	require.NoError(t, err)

	return file.Name()
}

func PatchManifest(t require.TestingT, applyPatch func(mf *model.Manifest), dir ...string) {
	mf, err := model.ReadManifest(dir...)
	require.NoError(t, err)

	applyPatch(mf)

	require.NoError(t, mf.Save(dir...))
}
