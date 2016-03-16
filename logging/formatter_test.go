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
}

type formattingTestCase struct {
	fields   Fields
	message  string
	expected string
}

func TestMessageFormatting(t *testing.T) {
	testCases := []formattingTestCase{
		{Fields{}, "a message with empty fields", "a message with empty fields"},
		{Fields{"key": "value"}, "message with one field", `message with one field key=value`},
		{Fields{"key": "value", "key2": "value2"}, "message with more than one field", `message with more than one field key=value, key2=value2`},
		{Fields{"keyz": "value", "keya": "value2"}, "message with non-alphabetic keys", `message with non-alphabetic keys keya=value2, keyz=value`},
	}

	for _, testCase := range testCases {
		assert.Equal(t, testCase.expected, fmtMessage(testCase.message, testCase.fields))
	}
}

func TestTextFormattingLevel(t *testing.T) {
	actual := textFormatter(levelInfo, "", "message", Fields{})
	assert.True(t, strings.Contains(actual, "[INFO]"))
}

func fmtMessage(message string, fields Fields) string {
	buf := &bytes.Buffer{}
	formatMessage(buf, message, fields)
	return buf.String()
}
