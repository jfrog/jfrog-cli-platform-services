//go:generate ${SCRIPTS_DIR}/gentz.sh

package model

import (
	"slices"
	"strings"
)

// For the timezone to be available we must call "make generate" the result must be committed

func IsValidTimezone(timezone string) bool {
	return slices.Index(TimeZones, strings.TrimSpace(timezone)) != -1
}
