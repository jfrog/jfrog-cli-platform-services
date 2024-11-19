package common

import "regexp"

var importPattern = regexp.MustCompile(`(?ms)^\s*(import\s+[^;]+;\s*)(.*)$`)

func CleanImports(source string) string {
	out := source
	match := importPattern.FindAllStringSubmatch(out, -1)
	for len(match) == 1 && len(match[0]) == 3 {
		out = match[0][2]
		match = importPattern.FindAllStringSubmatch(out, -1)
	}
	return out
}
