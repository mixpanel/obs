package logging

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggerWrites(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := testLogger(buf)
	logger.Infof("test", Fields{"key": "value"})
	assert.Contains(t, buf.String(), `key=value`)
}

func TestLoggerNamed(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := testLogger(buf)
	logger = logger.Named("new name")
	logger.Info("test")
	assert.Contains(t, buf.String(), "new name")
}

func TestSyslog(t *testing.T) {
	logger := newLogger(levelDebug, "", levelNever)
	buf := &bytes.Buffer{}
	logger.syslog = buf
	logger.syslogLevel = levelInfo

	logger.Infof("test", Fields{"key": "value"})
	if assert.Equal(t, "mixpanel ", buf.String()[:9]) {
		parsed := map[string]interface{}{}
		err := json.Unmarshal(buf.Bytes()[9:], &parsed)
		if assert.NoError(t, err) {
			expectedKeys := []string{"pid", "role", "argv", "executable", "key", "level", "logger", "message"}
			for _, k := range expectedKeys {
				v, found := parsed[k]
				assert.NotNil(t, v)
				assert.True(t, found)
			}
		}
	}
}

func testLogger(w io.Writer) Logger {
	logger := newLogger(levelNever, "", levelDebug)
	log.SetOutput(w)
	return logger
}

func resetLogOutput() {
	log.SetOutput(os.Stderr)
}
