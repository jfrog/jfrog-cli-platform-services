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

type addSecretCommand struct {
	ctx *components.Context
}

func GetAddSecretCommand() components.Command {
	return components.Command{
		Name:        "add-secret",
		Description: "Add a secret to a worker",
		AIDescription: `Add or update an encrypted secret in the local manifest.json. The secret value is encrypted with a user-supplied password and stored under manifest.secrets; the plaintext value never leaves the machine until 'jf worker deploy' (or 'jf worker test-run') unwraps it for transmission over TLS.

When to use:
- Storing an API token, password, or other sensitive value for a worker.
- Rotating an existing secret (with --edit).

Prerequisites:
- A valid manifest.json in the current directory.
- The same encryption password used by any previously added secret in this manifest (all secrets share one password).
- The secret value, either entered interactively or supplied via JFROG_WORKER_CLI_DEV_ADD_SECRET_VALUE.

Common patterns:
  $ jf worker add-secret api-token
  $ jf worker add-secret api-token --edit
  $ JFROG_WORKER_CLI_DEV_ADD_SECRET_VALUE=xyz jf worker add-secret api-token   # non-interactive

Gotchas:
- Without --edit, the command fails if a secret with the same name already exists.
- The encryption password is prompted interactively; reuse the same one for every secret in a given manifest or decryption will fail at deploy time.
- This command does NOT call the server; it only writes manifest.json. Run 'jf worker deploy' afterwards to push the secret.

Related: jf worker deploy, jf worker test-run`,
		Aliases:     []string{"as"},
		Flags: []components.Flag{
			components.NewBoolFlag(model.FlagEdit, "Whether to update an existing secret.", components.WithBoolDefaultValue(false)),
		},
		Arguments: []components.Argument{
			{
				Name:        "secret-name",
				Description: "The secret name.",
			},
		},
		Action: func(c *components.Context) error {
			cmd := &addSecretCommand{c}
			return cmd.run()
		},
	}
}

func (c *addSecretCommand) run() error {
	manifest, err := common.ReadManifest()
	if err != nil {
		return err
	}

	if err = common.ValidateManifest(manifest, nil); err != nil {
		return err
	}

	secretName, err := c.getSecretName()
	if err != nil {
		return err
	}

	err = c.checkUpdate(manifest, secretName)
	if err != nil {
		return err
	}

	encryptionKey, err := common.ReadSecretPassword()
	if err != nil {
		return err
	}

	secretValue, err := c.readSecretValue()
	if err != nil {
		return err
	}

	encryptedValue, err := common.EncryptSecret(encryptionKey, secretValue)
	if err != nil {
		return err
	}

	// We back the secrets up so that we do not have to encrypt them again
	existingEncryptedSecrets := model.Secrets{}
	for k, v := range manifest.Secrets {
		existingEncryptedSecrets[k] = v
	}

	if err = common.DecryptManifestSecrets(manifest, encryptionKey); err != nil {
		log.Debug("Cannot decrypt existing secrets: %+v", err)
		return fmt.Errorf("others secrets are encrypted with a different password, please use the same one")
	} else {
		manifest.Secrets = existingEncryptedSecrets
	}

	if manifest.Secrets == nil {
		manifest.Secrets = model.Secrets{secretName: encryptedValue}
	} else {
		manifest.Secrets[secretName] = encryptedValue
	}

	err = common.SaveManifest(manifest)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Secret '%s' saved", secretName))

	return nil
}

func (c *addSecretCommand) getSecretName() (string, error) {
	if len(c.ctx.Arguments) < 1 {
		return "", plugins_common.WrongNumberOfArgumentsHandler(c.ctx)
	}
	return c.ctx.Arguments[0], nil
}

func (c *addSecretCommand) checkUpdate(mf *model.Manifest, secretName string) error {
	_, exists := mf.Secrets[secretName]
	if exists && !c.ctx.GetBoolFlagValue(model.FlagEdit) {
		return fmt.Errorf("%s already exists, use --%s to overwrite", secretName, model.FlagEdit)
	}
	return nil
}

func (c *addSecretCommand) readSecretValue() (string, error) {
	secretValue, valueInEnv := os.LookupEnv(model.EnvKeyAddSecretValue)
	if valueInEnv {
		return secretValue, nil
	}

	return ioutils.ScanPasswordFromConsole("Value: ")
}
