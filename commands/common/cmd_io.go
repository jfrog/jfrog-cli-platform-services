package common

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
)

// Useful to capture output in tests
var (
	cliOut io.Writer = os.Stdout
	cliIn  io.Reader = os.Stdin
)

type InputReader struct {
	ctx *components.Context
}

func NewInputReader(ctx *components.Context) *InputReader {
	return &InputReader{ctx}
}

func NewCsvWriter() *csv.Writer {
	return csv.NewWriter(cliOut)
}

func Print(message string, args ...any) error {
	_, err := fmt.Fprintf(cliOut, message, args...)
	return err
}

func PrintJson(data []byte) error {
	_, err := cliOut.Write(PrettifyJson(data))
	return err
}

func printJsonOrLogError(data []byte) error {
	if _, writeErr := cliOut.Write(PrettifyJson(data)); writeErr != nil {
		log.Debug(fmt.Sprintf("Write error: %+v", writeErr))
	}
	return nil
}

func (c *InputReader) ReadData() (map[string]any, error) {
	if len(c.ctx.Arguments) == 0 {
		return nil, fmt.Errorf("missing json payload argument")
	}

	// The input should always be the last argument
	jsonPayload := c.ctx.Arguments[len(c.ctx.Arguments)-1]

	if jsonPayload == "-" {
		return c.ReadDataFromStdin()
	}

	if strings.HasPrefix(jsonPayload, "@") {
		return c.ReadDataFromFile(jsonPayload[1:])
	}

	return c.unmarshalData([]byte(jsonPayload))
}

func (c *InputReader) ReadDataFromStdin() (map[string]any, error) {
	data := map[string]any{}

	decoder := json.NewDecoder(cliIn)

	err := decoder.Decode(&data)
	if err != nil {
		return nil, err
	}

	return data, err
}

func (c *InputReader) ReadDataFromFile(filePath string) (map[string]any, error) {
	if filePath == "" {
		return nil, errors.New("missing file path")
	}

	dataBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return c.unmarshalData(dataBytes)
}

func (c *InputReader) unmarshalData(dataBytes []byte) (map[string]any, error) {
	data := map[string]any{}

	err := json.Unmarshal(dataBytes, &data)
	if err != nil {
		return nil, fmt.Errorf("invalid json payload: %w", err)
	}

	return data, nil
}

func CloseQuietly(c io.Closer) {
	if err := c.Close(); err != nil {
		log.Debug(fmt.Sprintf("Error closing resource: %+v", err))
	}
}
