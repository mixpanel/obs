package logging

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoggerWrites(t *testing.T) {
	logger, buf := testLogger(formatText)
	logger.Infof("test", Fields{"key": "value"})
	assert.Contains(t, buf.String(), `key=value`)
}

func TestLoggerNamed(t *testing.T) {
	logger, buf := testLogger(formatText)
	logger = logger.Named("new name")
	logger.Info("test")
	assert.Contains(t, buf.String(), "new name")
}

func TestSyslog(t *testing.T) {
	logger := newLogger(levelDebug, "", levelNever, formatText)
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

func TestLoggerJSON(t *testing.T) {
	logger, buf := testLogger(formatJSON)
	logger.Infof("test", Fields{"key": "value"})
	var res map[string]interface{}
	err := json.Unmarshal(buf.Bytes(), &res)
	assert.NoError(t, err)
	assert.Equal(t, "test", res["message"])
	assert.Equal(t, "value", res["key"])

}

func testLogger(format format) (Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	logger := newLogger(levelNever, "", levelDebug, format)
	log.SetOutput(buf)
	return logger, buf
}

func resetLogOutput() {
	log.SetOutput(os.Stderr)
}
