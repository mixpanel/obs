package logging

import (
	"fmt"
	"strings"
)

type level int

var (
	levelDebug    = level(10)
	levelInfo     = level(20)
	levelWarn     = level(30)
	levelError    = level(40)
	levelCritical = level(50)
	levelNever    = level(60)
)

func levelStringToLevel(str string) level {
	switch strings.ToUpper(str) {
	case "NEVER":
		return levelNever
	case "DEBUG":
		return levelDebug
	case "INFO":
		return levelInfo
	case "WARN":
		return levelWarn
	case "ERROR":
		return levelError
	case "CRITICAL":
		return levelCritical
	default:
		initError(fmt.Sprintf("Invalid log level %v.", str))
		return levelWarn
	}
}
