package search

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// FrontmatterEntry is a key-value pair extracted from YAML frontmatter.
// Arrays are expanded: tags: [a, b] becomes two entries {tags, a} and {tags, b}.
// Nested maps use dotted keys: author.name: Alice.
type FrontmatterEntry struct {
	Key   string
	Value string
}

// ParseFrontmatter parses raw YAML frontmatter into key-value entries.
func ParseFrontmatter(raw string) []FrontmatterEntry {
	if raw == "" {
		return nil
	}

	var data map[string]interface{}
	if err := yaml.Unmarshal([]byte(raw), &data); err != nil {
		return nil
	}

	var entries []FrontmatterEntry
	flatten("", data, &entries)
	return entries
}

// flatten recursively converts a nested map into dot-separated key-value entries.
func flatten(prefix string, data map[string]interface{}, entries *[]FrontmatterEntry) {
	for k, v := range data {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]interface{}:
			flatten(key, val, entries)
		case []interface{}:
			for _, item := range val {
				*entries = append(*entries, FrontmatterEntry{
					Key:   key,
					Value: fmt.Sprintf("%v", item),
				})
			}
		default:
			*entries = append(*entries, FrontmatterEntry{
				Key:   key,
				Value: fmt.Sprintf("%v", val),
			})
		}
	}
}
