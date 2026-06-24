package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbedMigrations(t *testing.T) {
	entries, err := embedMigrations.ReadDir(migrationsDir)
	require.NoError(t, err)
	require.NotEmpty(t, entries)

	foundInitial := false
	for _, entry := range entries {
		if entry.Name() == "00001_initial_schema.sql" {
			foundInitial = true
			break
		}
	}

	assert.True(t, foundInitial, "initial migration should be embedded")
}
