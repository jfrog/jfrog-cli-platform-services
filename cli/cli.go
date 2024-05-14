package cli

import (
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/workers-cli/commands"
)

func GetApp() components.App {
	app := components.App{}
	app.Name = "worker"
	app.Description = "Tools for managing workers"
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
	}
}
