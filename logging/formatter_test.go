package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLevelToString(t *testing.T) {
	assert.Equal(t, "DEBUG", levelToString(levelDebug))
	assert.Equal(t, "INFO", levelToString(levelInfo))
	assert.Equal(t, "WARN", levelToString(levelWarn))
	assert.Equal(t, "ERROR", levelToString(levelError))
	assert.Equal(t, "CRITICAL", levelToString(levelCritical))
}

type formattingTestCase struct {
	fields   Fields
	message  string
	expected string
}

func TestMessageFormatting(t *testing.T) {
	testCases := []formattingTestCase{
		{Fields{"key": "value", "key2": "value2"}, "a message with a {{key}} and something after", "a message with a value and something after"},
		{Fields{"key": "value", "key2": "value2"}, "a message with a {{key}} and another {{key2}} after", "a message with a value and another value2 after"},
		{Fields{"key": "value"}, "the value is \\{{key}}", "the value is \\{{key}}"},
		{Fields{"key": "value"}, "a message with a {{key}} and another {{key2}} after", "a message with a value and another {{key2}} after"},
		{Fields{"key": "value"}, "a message with no variables", "a message with no variables"},
		{Fields{"key": "value"}, "{{key}} is the value", "value is the value"},
		{Fields{"key": "value"}, "the value is {{key}}", "the value is value"},
	}

	for _, testCase := range testCases {
		assert.Equal(t, testCase.expected, fmtMessage(testCase.message, testCase.fields))
	}
}

func TestTextFormattingLevel(t *testing.T) {
	actual := textFormatter(levelInfo, "message", Fields{})
	assert.True(t, strings.Contains(actual, "[INFO]"))
}

func TestJSONFormatter(t *testing.T) {
	fields := Fields{"key": "value"}
	message := "a single {{key}}"
	var actual map[string]interface{}
	err := json.Unmarshal([]byte(jsonFormatter(levelDebug, message, fields)), &actual)
	assert.Nil(t, err)
	assert.Equal(t, "DEBUG", actual["level"])
	assert.Equal(t, "a single value", actual["message"])
}

func fmtMessage(message string, fields Fields) string {
	buf := &bytes.Buffer{}
	formatMessage(buf, message, fields)
	return buf.String()
}
