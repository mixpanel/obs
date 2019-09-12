package logging

var initErrors []string

func New(syslogLevel, fileLevel, filePath, format string) Logger {
	logger := buildLogger(syslogLevel, fileLevel, filePath, format)
	for _, message := range initErrors {
		logger.Error(message, nil)
	}
	initErrors = nil
	return logger
}

func buildLogger(syslogLevel, fileLevel, filePath, format string) Logger {
	return newLogger(
		levelStringToLevel(syslogLevel),
		filePath,
		levelStringToLevel(fileLevel),
		formatToEnum(format),
	)
}

func initError(message string) {
	initErrors = append(initErrors, message)
}
