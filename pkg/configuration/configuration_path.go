package configuration

import (
	"strings"
)

const KeyDelimiter = ":"

func pathSectionKey(path string) string {
	idx := strings.LastIndexByte(path, ':')
	if idx == -1 {
		return path
	}

	return path[idx+1:]
}

func getChildrenByPath(
	path string,
	providers []IConfigurationProvider,
	getSection func(string) IConfigurationSection,
) []IConfigurationSection {
	keySet := make(map[string]struct{})
	for _, provider := range providers {
		for _, key := range provider.GetChildKeys(path) {
			keySet[key] = struct{}{}
		}
	}

	sections := make([]IConfigurationSection, 0, len(keySet))
	if path != "" {
		for key := range keySet {
			sections = append(sections, getSection(path+KeyDelimiter+key))
		}
	} else {
		for key := range keySet {
			sections = append(sections, getSection(key))
		}
	}
	return sections
}
