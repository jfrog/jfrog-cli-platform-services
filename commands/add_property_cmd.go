// Package commands provides JFrog platform services worker management commands.
package commands

import (
	"fmt"
	"os"

	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

type addPropertyCommand struct {
	ctx *components.Context
}

func GetAddPropertyCommand() components.Command {
	return components.Command{
		Name:        "add-property",
		Description: "Add a clear-text property to a worker",
		AIDescription: `Add or update a clear-text property in the local manifest.json. Pass the value as a second argument, or omit it to prompt or use JFROG_WORKER_CLI_DEV_ADD_PROPERTY_VALUE. This command only writes locally; run 'jf worker deploy' to send the property to the server.

Properties are not secrets: values remain unencrypted and readable on disk. Omitting manifest.properties preserves remote properties, while an explicit empty object clears them on the next deploy.`,
		Aliases: []string{"ap"},
		Flags: []components.Flag{
			components.NewBoolFlag(model.FlagEdit, "Whether to update an existing property.", components.WithBoolDefaultValue(false)),
		},
		Arguments: []components.Argument{
			{
				Name:        "property-name",
				Description: "The property name.",
			},
			{
				Name:        "property-value",
				Description: "The property value. If omitted, prompted or taken from JFROG_WORKER_CLI_DEV_ADD_PROPERTY_VALUE.",
				Optional:    true,
			},
		},
		Action: func(c *components.Context) error {
			return (&addPropertyCommand{ctx: c}).run()
		},
	}
}

func (c *addPropertyCommand) run() error {
	manifest, err := common.ReadManifest()
	if err != nil {
		return err
	}
	if err = common.ValidateManifest(manifest, nil); err != nil {
		return err
	}

	propertyName, err := c.getPropertyName()
	if err != nil {
		return err
	}
	if err = c.checkUpdate(manifest, propertyName); err != nil {
		return err
	}

	propertyValue := c.readPropertyValue()
	if manifest.Properties == nil {
		manifest.Properties = map[string]string{}
	}
	manifest.Properties[propertyName] = propertyValue

	if err = common.SaveManifest(manifest); err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Property '%s' saved", propertyName))
	return nil
}

func (c *addPropertyCommand) getPropertyName() (string, error) {
	if len(c.ctx.Arguments) < 1 || len(c.ctx.Arguments) > 2 {
		return "", plugins_common.WrongNumberOfArgumentsHandler(c.ctx)
	}
	return c.ctx.Arguments[0], nil
}

func (c *addPropertyCommand) checkUpdate(manifest *model.Manifest, propertyName string) error {
	if _, exists := manifest.Properties[propertyName]; exists && !c.ctx.GetBoolFlagValue(model.FlagEdit) {
		return fmt.Errorf("%s already exists, use --%s to overwrite", propertyName, model.FlagEdit)
	}
	return nil
}

func (c *addPropertyCommand) readPropertyValue() string {
	if len(c.ctx.Arguments) > 1 {
		return c.ctx.Arguments[1]
	}
	if propertyValue, exists := os.LookupEnv(model.EnvKeyAddPropertyValue); exists {
		return propertyValue
	}

	var propertyValue string
	ioutils.ScanFromConsole("Value", &propertyValue, "")
	return propertyValue
}
