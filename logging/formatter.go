package logging

import (
	"bytes"
	"fmt"
	"os"
	"sort"
)

var myPid = os.Getpid()

func textFormatter(lvl level, name, message string, fields Fields) string {
	buffer := bytes.NewBuffer(make([]byte, 0, len(message)*2))

	if name == "" {
		fmt.Fprintf(buffer, "pid=%d [%s]: ", myPid, levelToString(lvl))
	} else {
		fmt.Fprintf(buffer, "pid=%d [%s] %s: ", myPid, levelToString(lvl), name)
	}
	formatMessage(buffer, message, fields)

	return buffer.String()
}

func formatMessage(buffer *bytes.Buffer, message string, fields Fields) {
	buffer.WriteString(message)

	if len(fields) == 0 {
		return
	}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	count, len := 0, len(keys)
	buffer.WriteByte(' ')
	for _, k := range keys {
		count++
		buffer.WriteString(k)
		buffer.WriteByte('=')
		fmt.Fprintf(buffer, "%v", fields[k])
		if count < len {
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
