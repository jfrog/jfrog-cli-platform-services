package common

import (
	"fmt"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

type ActionsMetadata []*model.ActionMetadata

func (c ActionsMetadata) ActionsNames() []string {
	names := make([]string, len(c))
	for i, action := range c {
		names[i] = action.Action.Name
	}
	return names
}

func (c ActionsMetadata) FindAction(actionName string, service ...string) (*model.ActionMetadata, error) {
	if len(c) == 0 {
		return nil, fmt.Errorf("no actions found")
	}

	application := ""
	if len(service) > 0 {
		application = service[0]
	}

	var match []*model.ActionMetadata

	for _, action := range c {
		if action.Action.Name == actionName && (application == "" || action.Action.Application == application) {
			match = append(match, action)
		}
	}

	if len(match) == 1 {
		return match[0], nil
	}

	if len(match) > 1 {
		return nil, fmt.Errorf("%d actions found with name '%s', please specify an application", len(match), actionName)
	}

	if application != "" {
		return nil, fmt.Errorf("action '%s' not found for application '%s'", actionName, application)
	}

	return nil, fmt.Errorf("action '%s' not found. It should be one of %s", actionName, c.ActionsNames())
}

func (c ActionsMetadata) ActionNeedsCriteria(actionName string, service ...string) bool {
	action, err := c.FindAction(actionName, service...)
	return err == nil && action != nil && action.MandatoryFilter
}
