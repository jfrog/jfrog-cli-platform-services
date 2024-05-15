package main

import (
	"github.com/jfrog/jfrog-cli-core/v2/plugins"
	"github.com/jfrog/jfrog-cli-platform-services/cli"
)

func main() {
	plugins.PluginMain(cli.GetWorkerApp())
}
