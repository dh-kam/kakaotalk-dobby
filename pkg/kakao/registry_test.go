package kakao

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientRegistry_BasicOperations(t *testing.T) {
	reg := NewClientRegistry()
	assert.Equal(t, 0, reg.Count())
	assert.Empty(t, reg.List())

	_, err := reg.GetDefault()
	require.ErrorIs(t, err, ErrNoBots)

	c1 := NewClient(ClientConfig{ClientID: "client-dobby"})
	c2 := NewClient(ClientConfig{ClientID: "client-infra"})

	// First registered bot automatically becomes default
	reg.Register("dobby", c1, false)
	assert.Equal(t, 1, reg.Count())
	assert.Equal(t, []string{"dobby"}, reg.List())

	defClient, err := reg.GetDefault()
	require.NoError(t, err)
	assert.Equal(t, c1, defClient)

	// Second bot registered
	reg.Register("infra", c2, false)
	assert.Equal(t, 2, reg.Count())
	assert.Equal(t, []string{"dobby", "infra"}, reg.List())

	// Default should still be dobby
	defClient, err = reg.GetDefault()
	require.NoError(t, err)
	assert.Equal(t, c1, defClient)

	// Retrieve by explicit names
	gotDobby, err := reg.Get("dobby")
	require.NoError(t, err)
	assert.Equal(t, c1, gotDobby)

	gotInfra, err := reg.Get("infra")
	require.NoError(t, err)
	assert.Equal(t, c2, gotInfra)

	// Case insensitive lookup
	gotInfraUpper, err := reg.Get("INFRA")
	require.NoError(t, err)
	assert.Equal(t, c2, gotInfraUpper)

	// Unknown bot error
	_, err = reg.Get("unknown")
	require.Error(t, err)
	require.ErrorIs(t, err, ErrBotNotFound)

	// SetDefault to infra
	err = reg.SetDefault("infra")
	require.NoError(t, err)

	defClient, err = reg.GetDefault()
	require.NoError(t, err)
	assert.Equal(t, c2, defClient)

	// SetDefault to unknown bot fails
	err = reg.SetDefault("nonexistent")
	require.ErrorIs(t, err, ErrBotNotFound)

	// Info inspection
	info := reg.Info()
	require.Len(t, info, 2)
	assert.Equal(t, "dobby", info[0].Name)
	assert.False(t, info[0].IsDefault)
	assert.Equal(t, "infra", info[1].Name)
	assert.True(t, info[1].IsDefault)
}

func TestBotTokenPath(t *testing.T) {
	assert.Equal(t, "/custom/path.json", BotTokenPath("infra", "/custom/path.json"))
	assert.Equal(t, DefaultTokenPath(), BotTokenPath("", ""))
	assert.Equal(t, DefaultTokenPath(), BotTokenPath("default", ""))

	infraPath := BotTokenPath("infra", "")
	assert.True(t, strings.HasSuffix(infraPath, "tokens_infra.json"))
}
