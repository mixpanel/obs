package metrics

import (
	"errors"
	"sort"
	"strings"
)

type metricType string

// Tags are additional metadata with the metric
// name, keep in mind that these can't be
// high cardinality
type Tags map[string]string

const (
	metricTypeCounter = metricType("ct")
	metricTypeStat    = metricType("h")
	metricTypeGauge   = metricType("g")
)

func formatName(prefix string, name string) string {
	formatted := prefix
	if len(name) > 0 && len(prefix) > 0 {
		formatted += "."
	}
	return formatted + name
}

// FormatTags is used by receivers and sinks to convert a map of tags into a string that can be
// used as a map key
func FormatTags(tags Tags) string {
	keys := make([]string, 0, len(tags))
	for key := range tags {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	formatted := ""

	for _, key := range keys {
		formatted += key + ":" + tags[key] + ","
	}

	return formatted
}

// ParseTags converts a string formatted using formatTags(see above) into a map of tags
func ParseTags(tagString string) (map[string]string, error) {
	split := strings.Split(tagString, ",")
	tags := make(map[string]string, len(split))

	for _, pair := range split {
		if len(pair) > 0 {
			entry := strings.Split(pair, ":")
			if len(entry) != 2 {
				return nil, errors.New("incorrectly formatted tag: " + pair)
			}
			tags[entry[0]] = entry[1]
		}
	}
	return tags, nil
}
