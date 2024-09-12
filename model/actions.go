package model

import (
	"fmt"
	"regexp"
)

const (
	ActionUnspecified          = "ACTION_UNSPECIFIED"
	ActionBeforeDownload       = "BEFORE_DOWNLOAD"
	ActionAfterDownload        = "AFTER_DOWNLOAD"
	ActionBeforeUpload         = "BEFORE_UPLOAD"
	ActionAfterCreate          = "AFTER_CREATE"
	ActionAfterBuildInfoSave   = "AFTER_BUILD_INFO_SAVE"
	ActionAfterMove            = "AFTER_MOVE"
	ActionGenericEvent         = "GENERIC_EVENT"
	ActionBeforeCreateToken    = "BEFORE_CREATE_TOKEN"
	ActionBeforePropertyCreate = "BEFORE_PROPERTY_CREATE"
)

var (
	actionsNames        = fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%s", ActionBeforeDownload, ActionAfterDownload, ActionBeforeUpload, ActionAfterCreate, ActionAfterBuildInfoSave, ActionAfterMove, ActionGenericEvent, ActionBeforeCreateToken, ActionBeforePropertyCreate)
	actionsNamesPattern = regexp.MustCompile("(" + actionsNames + ")")
)

var actionsWithoutCriteria = map[string]any{
	ActionAfterBuildInfoSave: struct{}{},
	ActionGenericEvent:       struct{}{},
	ActionBeforeCreateToken:  struct{}{},
}

func ActionNames() string {
	return actionsNames
}

func ActionNeedsCriteria(actionName string) bool {
	_, doNotNeedCriteria := actionsWithoutCriteria[actionName]
	return !doNotNeedCriteria
}

func ActionIsValid(actionName string) bool {
	return actionsNamesPattern.MatchString(actionName)
}
