//go:build itest

package commands

import "io"

func SetCliIn(reader io.Reader) {
	cliIn = reader
}

func SetCliOut(writer io.Writer) {
	cliOut = writer
}
