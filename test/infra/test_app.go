//go:build itest

package infra

import (
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-cli-platform-services/commands"
)

const AppName = "worker-test"

func getApp() components.App {
	app := components.App{}
	app.Name = AppName
	app.Description = "Provides tools for worker"
	app.Version = "v1.0.0"
	app.Commands = []components.Command{
		commands.GetInitCommand(),
		commands.GetDryRunCommand(),
		commands.GetDeployCommand(),
		commands.GetExecuteCommand(),
		commands.GetRemoveCommand(),
		commands.GetListCommand(),
		commands.GetAddSecretCommand(),
		commands.GetListEventsCommand(),
		commands.GetEditScheduleCommand(),
	}
	return app
}
