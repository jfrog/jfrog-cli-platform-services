package commands

import (
	"fmt"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

type editScheduleCommand struct {
	ctx *components.Context
}

const (
	flagScheduleCron     = "cron"
	flagScheduleTimezone = "timezone"
)

func GetEditScheduleCommand() components.Command {
	return components.Command{
		Name:        "edit-schedule",
		Description: "Edit the schedule criteria of a SCHEDULED_EVENT worker",
		AIDescription: `Update the cron expression and timezone of a SCHEDULED_EVENT worker's local manifest.json. The change is written to disk only; run 'jf worker deploy' afterwards to apply it on the server.

When to use:
- Changing the firing cadence of an existing SCHEDULED_EVENT worker.
- Adjusting the timezone (e.g. switching from UTC to America/New_York).
- Recovering from a malformed schedule by overwriting filterCriteria.schedule.

Prerequisites:
- A manifest.json in the current directory whose action is SCHEDULED_EVENT.
- A valid cron expression with minutes resolution (seconds resolution is not supported by the Worker service).
- (Optional) An IANA timezone name; defaults to UTC.

Common patterns:
  $ jf worker edit-schedule --cron "0 * * * *"
  $ jf worker edit-schedule --cron "*/15 * * * *" --timezone "America/New_York"
  $ jf worker edit-schedule --cron "0 9 * * 1-5" --timezone "Europe/Paris"

Gotchas:
- The command refuses to run if manifest.action is not SCHEDULED_EVENT.
- Cron expressions must have exactly 5 fields (minute hour day month dow); 6-field (with seconds) or less than 5 digits expressions are rejected.
- The change is local only — nothing is sent to the server until 'jf worker deploy'.

Related: jf worker deploy, jf worker init, jf worker list-event`,
		Aliases: []string{"es"},
		Flags: []components.Flag{
			components.NewStringFlag(flagScheduleCron, "A standard cron expression with minutes resolution. Seconds resolution is not supported by Worker service.", components.SetMandatory()),
			components.NewStringFlag(flagScheduleTimezone, "The timezone to use for scheduling.", components.WithStrDefaultValue("UTC")),
		},
		Action: func(c *components.Context) error {
			cmd := &editScheduleCommand{c}
			return cmd.run()
		},
	}
}

func (c *editScheduleCommand) run() error {
	manifest, err := common.ReadManifest()
	if err != nil {
		return err
	}

	if err = common.ValidateManifest(manifest, nil); err != nil {
		return err
	}

	if manifest.Action != "SCHEDULED_EVENT" {
		return fmt.Errorf("the worker is not a SCHEDULED_EVENT worker")
	}

	newCriteria := &model.ScheduleFilterCriteria{
		Cron:     c.ctx.GetStringFlagValue(flagScheduleCron),
		Timezone: c.ctx.GetStringFlagValue(flagScheduleTimezone),
	}

	if err = common.ValidateScheduleCriteria(newCriteria); err != nil {
		return fmt.Errorf("invalid schedule provided: %w", err)
	}

	manifest.FilterCriteria = &model.FilterCriteria{
		Schedule: newCriteria,
	}

	if err = common.SaveManifest(manifest); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	log.Info("Manifest updated successfully. Run 'jf worker deploy' to apply the changes.")

	return nil
}
