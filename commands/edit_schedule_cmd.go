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
		Aliases:     []string{"es"},
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

	newCriteria := model.ScheduleFilterCriteria{
		Cron:     c.ctx.GetStringFlagValue(flagScheduleCron),
		Timezone: c.ctx.GetStringFlagValue(flagScheduleTimezone),
	}

	if err = common.ValidateScheduleCriteria(&newCriteria); err != nil {
		return fmt.Errorf("invalid schedule provided: %w", err)
	}

	manifest.FilterCriteria.Schedule = newCriteria

	if err = common.SaveManifest(manifest); err != nil {
		return fmt.Errorf("failed to save manifest: %w", err)
	}

	log.Info("Manifest updated successfully. Run 'jf worker deploy' to apply the changes.")

	return nil
}
