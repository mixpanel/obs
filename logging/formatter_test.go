package logging

import (
	"bytes"
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
	assert.Equal(t, "UNKNOWN", levelToString(level(123245)))
}

type formattingTestCase struct {
	fields   Fields
	message  string
	expected string
}

func TestMessageFormatting(t *testing.T) {
	testCases := []formattingTestCase{
		{Fields{}, "a message with empty fields", "a message with empty fields"},
		{Fields{"key": "value"}, "message with one field", `message with one field | key=value`},
		{Fields{"key": "value", "key2": "value2"}, "message with more than one field", `message with more than one field | key=value, key2=value2`},
		{Fields{"keyz": "value", "keya": "value2"}, "message with non-alphabetic keys", `message with non-alphabetic keys | keya=value2, keyz=value`},
	}

	for _, testCase := range testCases {
		assert.Equal(t, testCase.expected, fmtMessage(testCase.message, testCase.fields))
	}
}

func TestTextFormattingLevel(t *testing.T) {
	actual := textFormatter(levelInfo, "", "message", Fields{})
	assert.True(t, strings.Contains(actual, "[INFO]"))
}

func TestFormatToEnum(t *testing.T) {
	assert.Equal(t, formatJSON, formatToEnum("json"))
	assert.Equal(t, formatText, formatToEnum("text"))
	assert.Panics(t, func() {
		formatToEnum("blah")
	})
}

func fmtMessage(message string, fields Fields) string {
	buf := &bytes.Buffer{}
	formatFields(buf, message, fields)
	return buf.String()
}
