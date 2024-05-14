package main

import (
	"github.com/jfrog/jfrog-cli-core/v2/plugins"
	"github.com/jfrog/workers-cli/cli"
)

func main() {
	plugins.PluginMain(cli.GetApp())
}
