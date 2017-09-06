package logging

// Null is the null implementation of logger.
var Null Logger = nullLogger(struct{}{})

type nullLogger struct{}

func (nl nullLogger) Debug(message string, fields Fields) {
}

func (nl nullLogger) Info(message string, fields Fields) {
}

func (nl nullLogger) Warn(message string, fields Fields) {
}

func (nl nullLogger) Error(message string, fields Fields) {
}

func (nl nullLogger) Critical(message string, fields Fields) {
}

func (nl nullLogger) IsDebug() bool {
	return false
}

func (nl nullLogger) IsInfo() bool {
	return false
}

func (nl nullLogger) IsWarn() bool {
	return false
}

func (nl nullLogger) IsError() bool {
	return false
}

func (nl nullLogger) IsCritical() bool {
	return false
}

func (nl nullLogger) Named(name string) Logger {
	return nl
}
