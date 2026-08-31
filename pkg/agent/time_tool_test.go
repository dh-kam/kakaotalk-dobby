package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentTimeTool(t *testing.T) {
	tool := &CurrentTimeTool{}
	assert.Equal(t, "get_current_time", tool.Name())
	assert.NotEmpty(t, tool.Description())
	assert.NotNil(t, tool.ParametersSchema())

	output, err := tool.Execute(context.Background(), "{}")
	require.NoError(t, err)
	assert.NotEmpty(t, output)

	var data map[string]interface{}
	err = json.Unmarshal([]byte(output), &data)
	require.NoError(t, err)

	assert.Contains(t, data, "iso8601")
	assert.Contains(t, data, "formatted")
	assert.Contains(t, data, "korean_formatted")
	assert.Contains(t, data, "date")
	assert.Contains(t, data, "time")
	assert.Contains(t, data, "weekday")
	assert.Contains(t, data, "timezone")
	assert.Equal(t, "Asia/Seoul (KST, UTC+9)", data["timezone"])
}
