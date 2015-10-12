package logging

var NullLogger Logger = nullLogger(struct{}{})

type nullLogger struct{}

func (nl nullLogger) Debugf(message string, fields Fields) {
}

func (nl nullLogger) Infof(message string, fields Fields) {
}

func (nl nullLogger) Warnf(message string, fields Fields) {
}

func (nl nullLogger) Errorf(message string, fields Fields) {
}

func (nl nullLogger) Criticalf(message string, fields Fields) {
}

func (nl nullLogger) Debug(message string) {
}

func (nl nullLogger) Info(message string) {
}

func (nl nullLogger) Warn(message string) {
}

func (nl nullLogger) Error(message string) {
}

func (nl nullLogger) Critical(message string) {
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
