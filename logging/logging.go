package logging

var initErrors []string

func New(syslogLevel, fileLevel, filePath string) Logger {
	logger := buildLogger(syslogLevel, fileLevel, filePath)
	for _, message := range initErrors {
		logger.Error(message)
	}
	initErrors = nil
	return logger
}

func buildLogger(syslogLevel, fileLevel, filePath string) Logger {
	return newLogger(
		levelStringToLevel(syslogLevel),
		filePath,
		levelStringToLevel(fileLevel),
	)
}

func initError(message string) {
	initErrors = append(initErrors, message)
}
