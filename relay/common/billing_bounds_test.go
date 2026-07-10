package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateTaskDurationBounds(t *testing.T) {
	assert.Nil(t, validateTaskDurationBounds(TaskSubmitReq{Duration: MaxTaskDurationSeconds}))
	assert.NotNil(t, validateTaskDurationBounds(TaskSubmitReq{Duration: -1}))
	assert.NotNil(t, validateTaskDurationBounds(TaskSubmitReq{Duration: MaxTaskDurationSeconds + 1}))
}
