package logging

import (
	"flag"
	"os"
)

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
	flagSet := flag.NewFlagSet("logging", flag.ContinueOnError)
	var syslogLevel, logFile, fileLevel string
	flagSet.StringVar(&syslogLevel, "log.syslog.level", "NEVER", "One of CRIT, ERR, WARN, INFO, DEBUG, NEVER. Defaults to WARN.")
	flagSet.StringVar(&fileLevel, "log.file.level", "INFO", "One of CRIT, ERR, WARN, INFO, DEBUG, NEVER. Defaults to INFO.")
	flagSet.StringVar(&logFile, "log.file.path", "", "File path to log. Defaults to stderr.")
	flagSet.Parse(os.Args)

	return newLogger(
		levelStringToLevel(syslogLevel),
		logFile,
		levelStringToLevel(fileLevel),
	)
}

func initError(message string) {
	initErrors = append(initErrors, message)
}
