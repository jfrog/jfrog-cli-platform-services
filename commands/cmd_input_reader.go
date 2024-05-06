package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
)

type cmdInputReader struct {
	ctx *components.Context
}

func (c *cmdInputReader) readData() (map[string]any, error) {
	if len(c.ctx.Arguments) == 0 {
		return nil, fmt.Errorf("missing json payload argument")
	}

	// The input should always be the last argument
	jsonPayload := c.ctx.Arguments[len(c.ctx.Arguments)-1]

	if jsonPayload == "-" {
		return c.readDataFromStdin()
	}

	if strings.HasPrefix(jsonPayload, "@") {
		return c.readDataFromFile(jsonPayload[1:])
	}

	return c.unmarshalData([]byte(jsonPayload))
}

func (c *cmdInputReader) readDataFromStdin() (map[string]any, error) {
	data := map[string]any{}

	decoder := json.NewDecoder(cliIn)

	err := decoder.Decode(&data)
	if err != nil {
		return nil, err
	}

	return data, err
}

func (c *cmdInputReader) readDataFromFile(filePath string) (map[string]any, error) {
	if filePath == "" {
		return nil, errors.New("missing file path")
	}

	dataBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return c.unmarshalData(dataBytes)
}

func (c *cmdInputReader) unmarshalData(dataBytes []byte) (map[string]any, error) {
	data := map[string]any{}

	err := json.Unmarshal(dataBytes, &data)
	if err != nil {
		return nil, fmt.Errorf("invalid json payload: %+v", err)
	}

	return data, nil
}
