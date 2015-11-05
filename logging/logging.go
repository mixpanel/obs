package logging

import "flags"

var initErrors []string

func NewDefault() Logger {
	logger := buildLogger()
	for _, message := range initErrors {
		logger.Error(message)
	}
	initErrors = nil
	return logger
}

func buildLogger() Logger {
	return newLogger(
		levelStringToLevel(flags.SyslogLevel),
		flags.LogFile,
		levelStringToLevel(flags.FileLevel),
	)
}

func initError(message string) {
	initErrors = append(initErrors, message)
}
