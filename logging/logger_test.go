package logging

import (
	"bytes"
	"io"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggerWrites(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := testLogger(buf)
	logger.Infof("test {{key}}", Fields{"key": "value"})
	assert.True(t, strings.Contains(buf.String(), "test value"))
}

func TestLoggerNamed(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := testLogger(buf)
	logger = logger.Named("new name")
	logger.Info("test")
	assert.True(t, strings.Contains(buf.String(), "new name"))
}

func TestSyslog(t *testing.T) {
	logger := newLogger(levelDebug, "", levelNever)
	buf := &bytes.Buffer{}
	logger.syslog = buf

	logger.Infof("test {{key}}", Fields{"key": "value"})
	assert.True(t, strings.Contains(buf.String(), "test value"))
}

func testLogger(w io.Writer) Logger {
	logger := newLogger(levelNever, "", levelDebug)
	log.SetOutput(w)
	return logger
}

func resetLogOutput() {
	log.SetOutput(os.Stderr)
}
