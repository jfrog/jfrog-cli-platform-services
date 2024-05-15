package commands

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/jfrog/jfrog-cli-platform-services/model"

	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

//go:embed templates/*
var templates embed.FS

func GetInitCommand() components.Command {
	return components.Command{
		Name:        "init",
		Description: "Initialize a worker",
		Aliases:     []string{"i"},
		Flags: []components.Flag{
			components.NewBoolFlag(model.FlagForce, "Whether or not to overwrite existing files"),
			model.GetNoTestFlag(),
		},
		Arguments: []components.Argument{
			{Name: "action", Description: fmt.Sprintf("The action that will trigger the worker (%s)", strings.Join(strings.Split(model.ActionNames(), "|"), ", "))},
			{Name: "worker-name", Description: "The name of the worker"},
		},
		Action: func(c *components.Context) error {
			if len(c.Arguments) < 2 {
				return fmt.Errorf("the action or worker name is missing, please see 'jf worker init --help'")
			}
			action := c.Arguments[0]
			workerName := c.Arguments[1]
			workingDir, err := os.Getwd()
			if err != nil {
				return err
			}
			if err := initWorker(workingDir, action, workerName, c.GetBoolFlagValue(model.FlagForce), c.GetBoolFlagValue(model.FlagNoTest)); err != nil {
				return err
			}
			log.Info(fmt.Sprintf("Worker %s initialized", workerName))
			return nil
		},
	}
}

func initWorker(targetDir string, action string, workerName string, force bool, skipTests bool) error {
	if !model.ActionIsValid(action) {
		return fmt.Errorf("invalid action '%s' action should be one of: %s", action, strings.Split(model.ActionNames(), "|"))
	}

	generate := initGenerator(targetDir, action, workerName, force, skipTests)

	if err := generate("package.json_template", "package.json"); err != nil {
		return err
	}

	if err := generate("tsconfig.json_template", "tsconfig.json"); err != nil {
		return err
	}

	if err := generate("manifest.json_template", "manifest.json"); err != nil {
		return err
	}

	if err := generate(action+".ts_template", "worker.ts"); err != nil {
		return err
	}

	if !skipTests {
		if err := generate(action+".spec.ts_template", "worker.spec.ts"); err != nil {
			return err
		}
	}

	return nil
}

func checkFileBeforeGenerate(filePath string, failIfExists bool) error {
	if _, err := os.Stat(filePath); err == nil || !errors.Is(err, os.ErrNotExist) {
		if failIfExists {
			return fmt.Errorf("%s already exists in %s, please use '--force' to overwrite if you know what you are doing", path.Base(filePath), path.Dir(filePath))
		}
		log.Warn(fmt.Sprintf("%s exists in %s. It will be overwritten", path.Base(filePath), path.Dir(filePath)))
	}
	return nil
}

func initGenerator(targetDir string, action string, workerName string, force bool, skipTests bool) func(string, string) error {
	return func(templateName, outputFilename string) error {
		tpl, err := template.New(templateName).ParseFS(templates, "templates/"+templateName)
		if err != nil {
			return err
		}

		filePath := path.Join(targetDir, outputFilename)

		err = checkFileBeforeGenerate(filePath, !force)
		if err != nil {
			return err
		}

		out, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
		if err != nil {
			return err
		}

		return tpl.Execute(out, map[string]any{
			"Action":      action,
			"WorkerName":  workerName,
			"HasCriteria": model.ActionNeedsCriteria(action),
			"HasTests":    !skipTests,
		})
	}
}
