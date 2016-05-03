package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	golog "log"
	"log/syslog"
	"os"
)

type Logger interface {
	Debugf(message string, fields Fields)
	Infof(message string, fields Fields)
	Warnf(message string, fields Fields)
	Errorf(message string, fields Fields)
	Criticalf(message string, fields Fields)

	Debug(message string)
	Info(message string)
	Warn(message string)
	Error(message string)
	Critical(message string)

	IsDebug() bool
	IsInfo() bool
	IsWarn() bool
	IsError() bool
	IsCritical() bool

	Named(name string) Logger
}

type logger struct {
	name          string
	syslog        io.Writer
	syslogLevel   level
	gologgerLevel level

	minLevel level
}

func newLogger(syslogLevel level, filepath string, fileLevel level) *logger {
	minLevel := syslogLevel
	if fileLevel < minLevel {
		minLevel = fileLevel
	}

	log := &logger{
		name:          "",
		minLevel:      minLevel,
		syslogLevel:   syslogLevel,
		gologgerLevel: fileLevel,
	}

	if syslogLevel != levelNever {
		syslogger, err := syslog.New(syslog.LOG_USER|syslog.LOG_NOTICE, "")
		if err != nil {
			initError(fmt.Sprintf("Unable to open syslog: %v.", err))
			log.syslogLevel = levelNever
		} else {
			log.syslog = syslogger
		}
	}

	if fileLevel == levelNever {
		golog.SetOutput(ioutil.Discard)
	} else if len(filepath) > 0 {
		file, err := os.OpenFile(filepath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
		if err != nil {
			initError(fmt.Sprintf("Unable to open file for logging: %v.", err))
			golog.SetOutput(os.Stderr)
		} else {
			golog.SetOutput(file)
		}
	} else {
		golog.SetOutput(os.Stderr)
	}

	return log
}

func (l *logger) Named(name string) Logger {
	return &logger{
		name:          name,
		syslog:        l.syslog,
		syslogLevel:   l.syslogLevel,
		gologgerLevel: l.gologgerLevel,
		minLevel:      l.minLevel,
	}
}

func (l *logger) Debug(message string) {
	l.Debugf(message, Fields{})
}

func (l *logger) Info(message string) {
	l.Infof(message, Fields{})
}

func (l *logger) Warn(message string) {
	l.Warnf(message, Fields{})
}

func (l *logger) Error(message string) {
	l.Errorf(message, Fields{})
}

func (l *logger) Critical(message string) {
	l.Criticalf(message, Fields{})
}

func (l *logger) Debugf(message string, fields Fields) {
	l.logAtLevel(levelDebug, message, fields)
}

func (l *logger) Infof(message string, fields Fields) {
	l.logAtLevel(levelInfo, message, fields)
}

func (l *logger) Warnf(message string, fields Fields) {
	l.logAtLevel(levelWarn, message, fields)
}

func (l *logger) Errorf(message string, fields Fields) {
	l.logAtLevel(levelError, message, fields)
}

func (l *logger) Criticalf(message string, fields Fields) {
	l.logAtLevel(levelCritical, message, fields)
}

func (l *logger) IsDebug() bool {
	return l.minLevel <= levelDebug
}

func (l *logger) IsInfo() bool {
	return l.minLevel <= levelInfo
}

func (l *logger) IsWarn() bool {
	return l.minLevel <= levelWarn
}

func (l *logger) IsError() bool {
	return l.minLevel <= levelError
}

func (l *logger) IsCritical() bool {
	return l.minLevel <= levelCritical
}

func (l *logger) logAtLevel(lvl level, message string, fields Fields) {
	if l.minLevel > lvl {
		return
	}

	formattedMessage := textFormatter(lvl, l.name, message, fields)

	if l.gologgerLevel <= lvl {
		golog.Println(formattedMessage)
	}
	if l.syslogLevel <= lvl {
		fields.Update(localhostFields)
		delete(fields, "hostname") // added by logstash
		fields["logger"] = l.name
		fields["level"] = levelToString(lvl)
		fields["message"] = formattedMessage

		payload, err := json.Marshal(fields)
		if err != nil {
			payload = []byte(`{"level": "ERROR", "message": "Failed to serialize to JSON."}`)
		}

		io.WriteString(l.syslog, "mixpanel "+string(payload))
	}
}
