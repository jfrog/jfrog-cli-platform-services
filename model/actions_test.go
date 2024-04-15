package model

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionNames(t *testing.T) {
	names := ActionNames()
	require.NotEmpty(t, names)
	matched, err := regexp.MatchString(`[A-B_|]+`, names)
	require.NoError(t, err)
	assert.True(t, matched)
}

func TestActionNeedsCriteria(t *testing.T) {
	for _, action := range strings.Split(ActionNames(), "|") {
		t.Run(action, func(t *testing.T) {
			assert.Equalf(t, action != "AFTER_BUILD_INFO_SAVE" && action != "GENERIC_EVENT", ActionNeedsCriteria(action), "ActionNeedsCriteria(%v)", action)
		})
	}
}

func TestActionIsValid(t *testing.T) {
	t.Run("HACK_ME", func(t *testing.T) {
		assert.Equalf(t, false, ActionIsValid("HACK_ME"), "ActionIsValid(%v)", "HACK_ME")
	})
	for _, action := range strings.Split(ActionNames(), "|") {
		t.Run(action, func(t *testing.T) {
			assert.Equalf(t, true, ActionIsValid(action), "ActionIsValid(%v)", action)
		})
	}
}
