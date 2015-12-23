package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
)

var textVariableReplacementRegex = regexp.MustCompile("{{[a-zA-Z0-9_]+}}")
var myPid = os.Getpid()

func textFormatter(lvl level, message string, fields Fields) string {
	buffer := bytes.NewBuffer(make([]byte, 0, len(message)*2))

	if fields["logger"] == "" {
		fmt.Fprintf(buffer, "pid=%d [%s]: ", myPid, levelToString(lvl))
	} else {
		fmt.Fprintf(buffer, "pid=%d [%s] %s: ", myPid, levelToString(lvl), fields["logger"])
	}
	formatMessage(buffer, message, fields)

	return buffer.String()
}

func formatMessage(buffer io.Writer, message string, fields Fields) {
	matches := textVariableReplacementRegex.FindAllStringIndex(message, -1)
	readerIndex := 0
	if matches != nil {
		for _, indices := range matches {
			start, end := indices[0], indices[1]
			if start != 0 && message[start-1:start] == "\\" {
				continue
			}

			key := message[start+2 : end-2]
			if value, ok := fields[key]; ok {
				io.WriteString(buffer, message[readerIndex:start])
				io.WriteString(buffer, fmt.Sprintf("%v", value))
				readerIndex = end
			}
		}
	}
	io.WriteString(buffer, message[readerIndex:])
}

func jsonFormatter(lvl level, message string, fields Fields) string {
	buffer := bytes.NewBuffer(make([]byte, 0, len(message)*2))
	formatMessage(buffer, message, fields)
	fields["message"] = buffer.String()
	fields["level"] = levelToString(lvl)
	delete(fields, "hostname") // added by logstash
	data, err := json.Marshal(fields)
	if err != nil {
		return `{"level": "ERROR", "message": "Failed to serialize to JSON."}`
	}
	return string(data)
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
