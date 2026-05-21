// Package cli provides the CLI entry point for JFrog platform services.
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
		AIDescription: "Manage JFrog Workers: TypeScript-based automations triggered by platform events (upload, download, generic events) or cron schedules. " +
			"Each worker is defined by a manifest.json plus a worker.ts source file in a local directory. " +
			"Typical lifecycle: 'jf worker init' to scaffold, 'jf worker test-run' to dry-run locally, 'jf worker deploy' to publish, " +
			"'jf worker execute' to invoke a GENERIC_EVENT worker, 'jf worker undeploy' to remove. " +
			"All commands require a JFrog Platform server configured via 'jf c add' or 'jf login' (or the JFROG_WORKER_CLI_DEV_* env vars).",
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
			commands.GetShowExecutionHistoryCommand(),
		},
	}
}
