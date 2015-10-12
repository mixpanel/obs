package logging

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLogLevelToString(t *testing.T) {
	assert.Equal(t, levelNever, levelStringToLevel("NEVER"))
	assert.Equal(t, levelDebug, levelStringToLevel("DEBUG"))
	assert.Equal(t, levelInfo, levelStringToLevel("INFO"))
	assert.Equal(t, levelWarn, levelStringToLevel("WARN"))
	assert.Equal(t, levelError, levelStringToLevel("ERROR"))
	assert.Equal(t, levelCritical, levelStringToLevel("CRITICAL"))
}
