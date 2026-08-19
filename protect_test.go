package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProtectBlocksDelete(t *testing.T) {
	dir := writeFixture(t)
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "protect", "b"}))

	a2, _, stderr := newTestApp(dir, nil)
	require.Equal(t, 1, a2.run([]string{"config", "delete-context", "b"}))
	assert.Contains(t, stderr.String(), "protected")
	assert.True(t, exists(a2.store().configPath("b")), "context must survive a refused delete")
}

func TestProtectForceStillDeletes(t *testing.T) {
	dir := writeFixture(t)
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "protect", "b"}))

	a2, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a2.run([]string{"config", "delete-context", "b", "--force"}))
	s := a2.store()
	assert.False(t, exists(s.configPath("b")))
	assert.False(t, exists(s.protectPath("b")), "the mark must not outlive the context")
}

func TestProtectBlocksRename(t *testing.T) {
	dir := writeFixture(t)
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "protect", "b"}))

	a2, _, stderr := newTestApp(dir, nil)
	require.Equal(t, 1, a2.run([]string{"config", "rename-context", "b", "c"}))
	assert.Contains(t, stderr.String(), "protected")
	assert.True(t, exists(a2.store().configPath("b")))
}

func TestProtectSurvivesRename(t *testing.T) {
	// Renaming with --force is allowed, but it must not quietly drop the mark.
	dir := writeFixture(t)
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "protect", "b"}))

	a2, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a2.run([]string{"config", "rename-context", "b", "c", "--force"}))
	s := a2.store()
	assert.False(t, s.isProtected("b"))
	assert.True(t, s.isProtected("c"), "protection must follow the context")
}

func TestUnprotectRestoresDelete(t *testing.T) {
	dir := writeFixture(t)
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "protect", "b"}))

	a2, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a2.run([]string{"config", "unprotect", "b"}))

	a3, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a3.run([]string{"config", "delete-context", "b"}))
	assert.False(t, exists(a3.store().configPath("b")))
}

func TestProtectIsIdempotent(t *testing.T) {
	dir := writeFixture(t)
	for _, want := range []string{"Protected", "already protected"} {
		a, _, stderr := newTestApp(dir, nil)
		require.Equal(t, 0, a.run([]string{"config", "protect", "b"}))
		assert.Contains(t, stderr.String(), want)
	}
}

func TestProtectUnknownContext(t *testing.T) {
	dir := writeFixture(t)
	a, _, _ := newTestApp(dir, nil)
	assert.Equal(t, 1, a.run([]string{"config", "protect", "nope"}))
}

func TestUnprotectedDeleteIsUnchanged(t *testing.T) {
	// The guard must stay out of the way when nothing is marked.
	dir := writeFixture(t)
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"config", "delete-context", "b"}))
	assert.False(t, exists(a.store().configPath("b")))
}
