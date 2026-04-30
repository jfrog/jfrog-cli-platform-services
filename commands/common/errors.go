package common

import (
	"fmt"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/common/format"
)

func ErrUnsupportedFormat(format format.OutputFormat, supportedFormats ...format.OutputFormat) error {
	supportedFormatsStr := make([]string, len(supportedFormats))
	for i, f := range supportedFormats {
		supportedFormatsStr[i] = string(f)
	}
	return fmt.Errorf("unsupported format '%s'. Accepted values: %s", format, strings.Join(supportedFormatsStr, ", "))
}
