//go:build itest

package infra

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

	"github.com/jfrog/jfrog-cli-platform-services/model"

	"github.com/google/uuid"
	corecommands "github.com/jfrog/jfrog-cli-core/v2/common/commands"
	"github.com/jfrog/jfrog-cli-core/v2/plugins"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/require"
)

type TestFunction func(it *Test)

type TestDefinition struct {
	Name          string
	Only          bool
	Skip          bool
	Input         string
	CaptureOutput bool
	Test          TestFunction
}

type Test struct {
	ServerId    string
	AccessToken string
	dataDir     string
	platformUrl string
	output      *bytes.Buffer
	t           *testing.T
}

const (
	requestTimeout      = 5 * time.Second
	defaultRetryBackoff = 250 * time.Millisecond
	defaultRetryTimeout = 2 * time.Second
)

var defaultRetryCommandOnErrors = []string{
	"returned an unexpected status code 502",
}

var runPlugin = plugins.RunCliWithPlugin(getApp())

func RunITests(tests []TestDefinition, t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	defer http.DefaultClient.CloseIdleConnections()

	var containsOnly *TestDefinition
	for _, tt := range tests {
		if tt.Only {
			containsOnly = &tt //nolint:exportability // We are good with pointing to 'tt' as 'break' is used.
			break
		}
	}

	if containsOnly != nil {
		tests = []TestDefinition{*containsOnly}
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.Skip {
				t.Skip("Skipping ", tt.Name)
			}
			runTest(t, tt)
		})
	}
}

func runTest(t *testing.T, testSpec TestDefinition) {
	homeDir := t.TempDir()
	// Setup cli home for tests
	err := os.Setenv(coreutils.HomeDir, homeDir)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := os.Unsetenv(coreutils.HomeDir)
		if err != nil {
			t.Logf("Error unsetting env '%s': %+v", coreutils.HomeDir, err)
		}
	})

	serverId := uuid.NewString()

	platformUrl := os.Getenv("JF_PLATFORM_URL")
	require.NotEmpty(t, platformUrl, "No platform URL provided, please set JF_PLATFORM_URL env var")

	accessToken := os.Getenv("JF_PLATFORM_ACCESS_TOKEN")
	require.NotEmpty(t, accessToken, "No platform token provided, please set JF_PLATFORM_ACCESS_TOKEN env var")

	// Generates a server config
	configCmd := corecommands.NewConfigCommand(corecommands.AddOrEdit, serverId)
	configCmd.SetInteractive(false)
	configCmd.SetMakeDefault(true)
	configCmd.SetEncPassword(false)
	configCmd.SetDetails(&config.ServerDetails{
		Url:         platformUrl,
		AccessToken: accessToken,
	})
	require.NoError(t, configCmd.Run())

	it := &Test{
		t:           t,
		ServerId:    serverId,
		AccessToken: accessToken,
		dataDir:     homeDir,
		platformUrl: platformUrl,
	}

	if testSpec.Input != "" {
		common.SetCliIn(bytes.NewReader([]byte(testSpec.Input)))
		t.Cleanup(func() {
			common.SetCliIn(os.Stdin)
		})
	}

	if testSpec.CaptureOutput {
		var newOutput bytes.Buffer
		common.SetCliOut(&newOutput)
		t.Cleanup(func() {
			common.SetCliOut(os.Stdout)
		})
		it.output = &newOutput
	}

	testSpec.Test(it)
}

func (it *Test) PrepareWorkerTestDir() (string, string) {
	dir, err := os.MkdirTemp("", "worker-*-init")
	require.NoError(it, err)

	it.Cleanup(func() {
		_ = os.RemoveAll(dir)
	})

	oldPwd, err := os.Getwd()
	require.NoError(it, err)

	err = os.Chdir(dir)
	require.NoError(it, err)

	it.Cleanup(func() {
		require.NoError(it, os.Chdir(oldPwd))
	})

	workerName := path.Base(dir)

	return dir, workerName
}

func (it *Test) RunCommand(args ...string) error {
	return it.RetryCommand(args, defaultRetryBackoff, defaultRetryTimeout, defaultRetryCommandOnErrors)
}

func (it *Test) RetryCommand(args []string, backoff time.Duration, timeout time.Duration, onErrorContaining []string) error {
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()
	os.Args = args

	start := time.Now()
	err := runPlugin()
	elapsed := time.Since(start)

	waitDuration := max(backoff, 250*time.Millisecond)

	for shouldRetryCommandOnError(err, onErrorContaining) && elapsed < timeout {
		time.Sleep(waitDuration)
		err = runPlugin()
		elapsed = time.Since(start)
	}

	return err
}

func shouldRetryCommandOnError(err error, onErrorContaining []string) bool {
	if err == nil || len(onErrorContaining) == 0 {
		return false
	}

	for _, s := range onErrorContaining {
		if strings.Contains(err.Error(), s) {
			return true
		}
	}

	return false
}

func (it *Test) CapturedOutput() []byte {
	if it.output != nil {
		return it.output.Bytes()
	}
	return nil
}

func (it *Test) GetAllWorkers() []*model.WorkerDetails {
	ctx, cancelCtx := context.WithTimeout(context.Background(), requestTimeout)
	defer cancelCtx()

	response := struct {
		Workers []*model.WorkerDetails `json:"workers"`
	}{}

	it.NewHttpRequestWithContext(ctx).
		WithAccessToken().
		Get("/worker/api/v1/workers").
		WithRetries(defaultRetryBackoff, defaultRetryTimeout, http.StatusBadGateway).
		Do().
		AsObject(&response)

	return response.Workers
}

func (it *Test) CreateWorker(createRequest *model.WorkerDetails) {
	ctx, cancelCtx := context.WithTimeout(context.Background(), requestTimeout)
	defer cancelCtx()

	it.Logf("Adding worker %s", createRequest.Key)

	jsonBytes, err := json.Marshal(createRequest)
	require.NoError(it, err)

	it.NewHttpRequestWithContext(ctx).
		WithAccessToken().
		WithJsonBytes(jsonBytes).
		Post("/worker/api/v1/workers").
		WithRetries(defaultRetryBackoff, defaultRetryTimeout, http.StatusBadGateway).
		Do().
		IsCreated()
}

func (it *Test) DeleteWorker(workerKey string) {
	ctx, cancelCtx := context.WithTimeout(context.Background(), requestTimeout)
	defer cancelCtx()

	it.Logf("Deleting worker '%s'", workerKey)

	status := it.NewHttpRequestWithContext(ctx).
		WithAccessToken().
		Delete("/worker/api/v1/workers/"+workerKey).
		WithRetries(defaultRetryBackoff, defaultRetryTimeout, http.StatusBadGateway).
		Do().
		StatusCode()

	if status != http.StatusNoContent {
		it.Logf("Delete worker '%s' failed with status %d", workerKey, status)
	} else {
		it.Logf("Deleted worker '%s'", workerKey)
	}
}

func (it *Test) DeleteAllWorkers() {
	for _, wk := range it.GetAllWorkers() {
		it.DeleteWorker(wk.Key)
	}
}

func (it *Test) Errorf(format string, args ...interface{}) {
	it.t.Errorf(format, args...)
}

func (it *Test) Logf(format string, args ...interface{}) {
	it.t.Logf(format, args...)
}

func (it *Test) FailNow() {
	it.t.FailNow()
}

func (it *Test) Skip() {
	it.t.Skip()
}

func (it *Test) Cleanup(f func()) {
	it.t.Cleanup(f)
}

func (it *Test) SkipBecause(reason string) {
	it.t.Skipf(reason)
}

func (it *Test) Helper() {
	it.t.Helper()
}

func (it *Test) Run(name string, f func(t *Test)) bool {
	return it.t.Run(name, func(t *testing.T) {
		f(&Test{t: t, ServerId: it.ServerId, AccessToken: it.AccessToken, dataDir: it.dataDir, output: it.output})
	})
}

func (it *Test) NewHttpRequest() *HttpRequest {
	return &HttpRequest{
		it:      it,
		url:     it.platformUrl,
		headers: make(map[string]string),
	}
}

func (it *Test) NewHttpRequestWithContext(ctx context.Context) *HttpRequest {
	r := &HttpRequest{
		it:         it,
		url:        it.platformUrl,
		headers:    make(map[string]string),
		reqContext: ctx,
	}
	return r
}
