package commands

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"text/template"

	plugins_common "github.com/jfrog/jfrog-cli-core/v2/plugins/common"
	"github.com/jfrog/jfrog-cli-platform-services/commands/common"

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
			plugins_common.GetServerIdFlag(),
			model.GetProjectKeyFlag(),
			model.GetApplicationFlag(),
			model.GetNoTestFlag(),
			model.GetTimeoutFlag(),
			components.NewBoolFlag(model.FlagForce, "Whether or not to overwrite existing files"),
		},
		Arguments: []components.Argument{
			{Name: "action", Description: "The action that will trigger the worker. Use `jf worker list-event` to see the list of available actions."},
			{Name: "worker-name", Description: "The name of the worker"},
		},
		Action: func(c *components.Context) error {
			return (&initHandler{c}).run()
		},
	}
}

type initHandler struct {
	*components.Context
}

func (c *initHandler) run() error {
	if len(c.Arguments) < 2 {
		return fmt.Errorf("the action or worker name is missing, please see 'jf worker init --help'")
	}

	action := c.Arguments[0]
	workerName := c.Arguments[1]

	workingDir, err := os.Getwd()
	if err != nil {
		return err
	}

	projectKey := c.GetStringFlagValue(model.FlagProjectKey)
	force := c.GetBoolFlagValue(model.FlagForce)
	skipTests := c.GetBoolFlagValue(model.FlagNoTest)

	err = c.initWorker(workingDir, action, workerName, projectKey, force, skipTests)
	if err != nil {
		return err
	}

	log.Info(fmt.Sprintf("Worker %s initialized", workerName))
	return nil
}

func (c *initHandler) initWorker(targetDir string, action string, workerName string, projectKey string, force bool, skipTests bool) error {
	server, err := model.GetServerDetails(c.Context)
	if err != nil {
		return err
	}

	actionsMeta, err := common.FetchActions(c.Context, server.Url, server.AccessToken, projectKey)
	if err != nil {
		return err
	}

	application := c.GetStringFlagValue(model.FlagApplication)

	actionMeta, err := actionsMeta.FindAction(action, application)
	if err != nil {
		log.Debug(fmt.Sprintf("Cannot not find action '%s': %+v", action, err))
		return fmt.Errorf("invalid action '%s' action should be one of: %s", action, actionsMeta.ActionsNames())
	}

	generate := c.initGenerator(targetDir, workerName, projectKey, force, skipTests, actionMeta)

	if err := generate("package.json_template", "package.json"); err != nil {
		return err
	}

	if err := generate("tsconfig.json_template", "tsconfig.json"); err != nil {
		return err
	}

	if err := generate("manifest.json_template", "manifest.json"); err != nil {
		return err
	}

	if err := generate("worker.ts_template", "worker.ts"); err != nil {
		return err
	}

	if !skipTests {
		if err := generate("worker.spec.ts_template", "worker.spec.ts"); err != nil {
			return err
		}
	}

	if err := c.generateTypesFile(targetDir, actionMeta, force); err != nil {
		return err
	}

	return nil
}

func (c *initHandler) checkFileBeforeGenerate(filePath string, failIfExists bool) error {
	if _, err := os.Stat(filePath); err == nil || !errors.Is(err, os.ErrNotExist) {
		if failIfExists {
			return fmt.Errorf("%s already exists in %s, please use '--force' to overwrite if you know what you are doing", path.Base(filePath), path.Dir(filePath))
		}
		log.Warn(fmt.Sprintf("%s exists in %s. It will be overwritten", path.Base(filePath), path.Dir(filePath)))
	}
	return nil
}

func (c *initHandler) initGenerator(targetDir string, workerName string, projectKey string, force bool, skipTests bool, md *model.ActionMetadata) func(string, string) error {
	params := map[string]any{
		"Action":                md.Action.Name,
		"Application":           md.Action.Application,
		"WorkerName":            workerName,
		"HasRepoFilterCriteria": md.MandatoryFilter && md.FilterType == model.FilterTypeRepo,
		"HasTests":              !skipTests,
		"HasRequestType":        md.ExecutionRequestType != "",
		"ExecutionRequestType":  md.ExecutionRequestType,
		"ProjectKey":            projectKey,
		"SourceCode":            md.SampleCode,
	}

	usedTypes := common.ExtractActionUsedTypes(md)
	if len(usedTypes) > 0 {
		params["UsedTypes"] = strings.Join(usedTypes, ", ")
	}

	return func(templateName, outputFilename string) error {
		tpl, err := template.New(templateName).ParseFS(templates, "templates/"+templateName)
		if err != nil {
			return err
		}

		filePath := path.Join(targetDir, outputFilename)

		err = c.checkFileBeforeGenerate(filePath, !force)
		if err != nil {
			return err
		}

		out, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
		if err != nil {
			return err
		}

		defer common.CloseQuietly(out)

		return tpl.Execute(out, params)
	}
}

func (c *initHandler) generateTypesFile(targetDir string, actionMeta *model.ActionMetadata, force bool) error {
	typesFilePath := path.Join(targetDir, "types.ts")

	err := c.checkFileBeforeGenerate(typesFilePath, !force)
	if err != nil {
		return err
	}

	out, err := os.OpenFile(typesFilePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}

	defer common.CloseQuietly(out)

	_, err = out.WriteString(common.AddExportToTypesDeclarations(actionMeta.TypesDefinitions))
	if err != nil {
		return err
	}

	return err
}
