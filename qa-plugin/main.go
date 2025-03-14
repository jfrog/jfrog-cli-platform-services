package main

import (
	"github.com/jfrog/jfrog-cli-core/v2/plugins"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-cli-platform-services/commands"
)

func main() {
	plugins.PluginMain(getApp())
}

func getApp() components.App {
	app := components.App{}
	app.Name = "worker-qa"
	app.Description = "Provides tools for worker [QA]"
	app.Version = "v1.0.0"
	app.Commands = getCommands()
	return app
}

func getCommands() []components.Command {
	return []components.Command{
		commands.GetInitCommand(),
		commands.GetDryRunCommand(),
		commands.GetDeployCommand(),
		commands.GetExecuteCommand(),
		commands.GetRemoveCommand(),
		commands.GetListCommand(),
		commands.GetAddSecretCommand(),
		commands.GetListEventsCommand(),
		commands.GetEditScheduleCommand(),
		commands.GetShowExecutionHistoryCommand(),
	}
}
