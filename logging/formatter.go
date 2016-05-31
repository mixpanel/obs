package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type format int

const (
	formatJSON = format(iota)
	formatText
)

var myPid = os.Getpid()

func jsonFormatter(lvl level, name, message string, fields Fields) string {
	fields.Update(localhostFields)
	delete(fields, "hostname") // added automatically
	fields["logger"] = name
	fields["level"] = levelToString(lvl)
	fields["message"] = message

	formatted, err := json.Marshal(fields)
	if err != nil {
		return `{"level": "ERROR", "message": "Failed to serialize to JSON."}`
	}
	return string(formatted)
}

func textFormatter(lvl level, name, message string, fields Fields) string {
	buffer := bytes.NewBuffer(make([]byte, 0, len(message)*2))

	if name == "" {
		fmt.Fprintf(buffer, "pid=%d [%s]: ", myPid, levelToString(lvl))
	} else {
		fmt.Fprintf(buffer, "pid=%d [%s] %s: ", myPid, levelToString(lvl), name)
	}
	formatFields(buffer, message, fields)

	return buffer.String()
}

func formatFields(buffer *bytes.Buffer, message string, fields Fields) {
	buffer.WriteString(message)

	if len(fields) == 0 {
		return
	}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	count := 0
	if len(keys) > 0 {
		buffer.WriteString(" | ")
	}
	for _, k := range keys {
		buffer.WriteString(k)
		buffer.WriteByte('=')
		fmt.Fprintf(buffer, "%v", fields[k])

		count++
		if count < len(keys) {
			buffer.WriteString(", ")
		}
	}
}

func levelToString(lvl level) string {
	switch lvl {
	case levelDebug:
		return "DEBUG"
	case levelInfo:
		return "INFO"
	case levelWarn:
		return "WARN"
	case levelError:
		return "ERROR"
	case levelCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

func formatToEnum(s string) format {
	switch s {
	case "json":
		return formatJSON
	case "text":
		return formatText
	default:
		panic(fmt.Errorf("error unknown log format type: %s", s))
	}
}
