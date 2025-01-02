package cli

import (
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-cli-platform-services/commands"
)

const category = "Platform Services"

func GetPlatformServicesApp() components.App {
	return components.CreateEmbeddedApp(
		category,
		nil,
		getWorkerNamespace(),
	)
}

func getWorkerNamespace() components.Namespace {
	return components.Namespace{
		Name:        "worker",
		Description: "Tools for managing workers",
		Category:    category,
		Commands: []components.Command{
			commands.GetInitCommand(),
			commands.GetDryRunCommand(),
			commands.GetDeployCommand(),
			commands.GetExecuteCommand(),
			commands.GetRemoveCommand(),
			commands.GetListCommand(),
			commands.GetAddSecretCommand(),
			commands.GetListEventsCommand(),
			commands.GetEditScheduleCommand(),
		},
	}
}
