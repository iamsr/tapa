package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootCommand(t *testing.T) {
	cmd := newRootCommand()

	assert.Equal(t, "tapa", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
}

func TestRootCommand_Version(t *testing.T) {
	cmd := newRootCommand()
	cmd.SetArgs([]string{"--version"})

	output := new(bytes.Buffer)
	cmd.SetOut(output)

	err := cmd.Execute()
	assert.NoError(t, err)
	assert.Contains(t, output.String(), "tapa version")
}

func TestRootCommand_HasAnalyzeSubcommand(t *testing.T) {
	cmd := newRootCommand()

	analyzeCmd, _, err := cmd.Find([]string{"analyze"})
	require.NoError(t, err)
	assert.Contains(t, analyzeCmd.Use, "analyze")
}

func TestRootCommand_ConfigFlag(t *testing.T) {
	cmd := newRootCommand()

	configFlag := cmd.PersistentFlags().Lookup("config")
	require.NotNil(t, configFlag)
	assert.Equal(t, "", configFlag.DefValue)
}
