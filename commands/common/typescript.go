package common

import (
	"regexp"
	"slices"
	"strings"

	"github.com/jfrog/jfrog-cli-platform-services/model"
)

var (
	tsNewInstancePattern   = regexp.MustCompile(`new\s+([A-Z][a-zA-Z0-9]+)\(`)
	tsFieldTypePattern     = regexp.MustCompile(`[a-zA-Z0-9]+\s*:\s*([A-Z][a-zA-Z0-9]+)`)
	tsFieldAccessPattern   = regexp.MustCompile(`\W([A-Z][a-zA-Z0-9]+)\.[a-zA-Z0-9]+`)
	tsTypeInTypeParameters = regexp.MustCompile(`<([A-Z][a-zA-Z0-9]+)>`)
	tsExcludeTypes         = []string{"PlatformContext"}
	tsUnexportedType       = regexp.MustCompile(`^(class|type|interface|enum|const)\s+[A-Za-z_$][0-9A-Za-z_$]*`)
)

// AddExportToTypesDeclarations Add export to (interface, class, enum, type) XXX found in the source.
func AddExportToTypesDeclarations(tsSource string) string {
	lines := strings.Split(tsSource, "\n")
	for i, line := range lines {
		if tsUnexportedType.MatchString(strings.TrimSpace(line)) {
			lines[i] = "export " + line
		}
	}
	return strings.Join(lines, "\n")
}

// ExtractActionUsedTypes extracts all the type used in an action's sampleCode and defined in the action's typesDefinitions.
func ExtractActionUsedTypes(md *model.ActionMetadata) []string {
	var types []string
	for _, typeName := range ExtractUsedTypes(md.SampleCode) {
		if strings.Contains(md.TypesDefinitions, typeName) {
			types = append(types, typeName)
		}
	}
	return types
}

// ExtractUsedTypes extracts types from a TypeScript source file.
func ExtractUsedTypes(tsSource string) []string {
	var types []string

	handleMatch := func(m string) {
		if slices.Index(types, m) == -1 && slices.Index(tsExcludeTypes, m) == -1 {
			types = append(types, m)
		}
	}

	extractTypesByPattern(tsNewInstancePattern, tsSource, handleMatch)
	extractTypesByPattern(tsFieldTypePattern, tsSource, handleMatch)
	extractTypesByPattern(tsFieldAccessPattern, tsSource, handleMatch)
	extractTypesByPattern(tsTypeInTypeParameters, tsSource, handleMatch)

	slices.Sort(types)

	return types
}

func extractTypesByPattern(pattern *regexp.Regexp, source string, onMatch func(m string)) {
	matches := pattern.FindAllStringSubmatch(source, -1)
	for _, match := range matches {
		onMatch(match[1])
	}
}
