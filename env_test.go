package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvWithCapturedADC(t *testing.T) {
	dir := writeFixture(t)
	adcB := filepath.Join(dir, "adc-b.json")
	cases := []struct {
		shell string
		want  string
	}{
		{"posix", fmt.Sprintf("export CLOUDSDK_ACTIVE_CONFIG_NAME='b'\nexport GOOGLE_APPLICATION_CREDENTIALS='%s'\n", adcB)},
		{"fish", fmt.Sprintf("set -gx CLOUDSDK_ACTIVE_CONFIG_NAME 'b'\nset -gx GOOGLE_APPLICATION_CREDENTIALS '%s'\n", adcB)},
		{"powershell", fmt.Sprintf("$env:CLOUDSDK_ACTIVE_CONFIG_NAME = 'b'\n$env:GOOGLE_APPLICATION_CREDENTIALS = '%s'\n", adcB)},
		{"nu", fmt.Sprintf("{\"CLOUDSDK_ACTIVE_CONFIG_NAME\":\"b\",\"GOOGLE_APPLICATION_CREDENTIALS\":%q}\n", adcB)},
	}
	for _, tc := range cases {
		t.Run(tc.shell, func(t *testing.T) {
			a, stdout, stderr := newTestApp(dir, nil)
			require.Equal(t, 0, a.run([]string{"env", "b", "--shell", tc.shell}))
			assert.Equal(t, tc.want, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}

func TestEnvWithoutCapturedADC(t *testing.T) {
	dir := writeFixture(t)
	cases := []struct {
		shell string
		want  string
	}{
		{"posix", "export CLOUDSDK_ACTIVE_CONFIG_NAME='a'\nunset GOOGLE_APPLICATION_CREDENTIALS\n"},
		{"fish", "set -gx CLOUDSDK_ACTIVE_CONFIG_NAME 'a'\nset -e GOOGLE_APPLICATION_CREDENTIALS\n"},
		{"powershell", "$env:CLOUDSDK_ACTIVE_CONFIG_NAME = 'a'\nRemove-Item Env:GOOGLE_APPLICATION_CREDENTIALS -ErrorAction Ignore\n"},
		{"nu", "{\"CLOUDSDK_ACTIVE_CONFIG_NAME\":\"a\",\"GOOGLE_APPLICATION_CREDENTIALS\":null}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.shell, func(t *testing.T) {
			a, stdout, stderr := newTestApp(dir, nil)
			require.Equal(t, 0, a.run([]string{"env", "a", "--shell", tc.shell}))
			assert.Equal(t, tc.want, stdout.String())
			assert.Empty(t, stderr.String())
		})
	}
}

func TestEnvWarnsAboutWellKnownFallback(t *testing.T) {
	dir := writeFixture(t)
	mustWrite(t, filepath.Join(dir, "application_default_credentials.json"), "{}")
	a, stdout, stderr := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"env", "a", "--shell", "posix"}))
	assert.True(t, strings.HasPrefix(stdout.String(), "export CLOUDSDK_ACTIVE_CONFIG_NAME='a'\n"))
	assert.Contains(t, stderr.String(), "gctx adc login a")
	assert.Contains(t, stderr.String(), "application_default_credentials.json")
}

func TestEnvReset(t *testing.T) {
	dir := writeFixture(t)
	cases := []struct {
		shell string
		want  string
	}{
		{"posix", "unset CLOUDSDK_ACTIVE_CONFIG_NAME\nunset GOOGLE_APPLICATION_CREDENTIALS\n"},
		{"fish", "set -e CLOUDSDK_ACTIVE_CONFIG_NAME\nset -e GOOGLE_APPLICATION_CREDENTIALS\n"},
		{"powershell", "Remove-Item Env:CLOUDSDK_ACTIVE_CONFIG_NAME -ErrorAction Ignore\nRemove-Item Env:GOOGLE_APPLICATION_CREDENTIALS -ErrorAction Ignore\n"},
		{"nu", "{\"CLOUDSDK_ACTIVE_CONFIG_NAME\":null,\"GOOGLE_APPLICATION_CREDENTIALS\":null}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.shell, func(t *testing.T) {
			a, stdout, _ := newTestApp(dir, nil)
			require.Equal(t, 0, a.run([]string{"env", "--reset", "--shell", tc.shell}))
			assert.Equal(t, tc.want, stdout.String())
		})
	}
}

func TestEnvUnknownContextEmitsNothing(t *testing.T) {
	dir := writeFixture(t)
	a, stdout, stderr := newTestApp(dir, nil)
	require.Equal(t, 1, a.run([]string{"env", "nosuch", "--shell", "posix"}))
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "nosuch")
	assert.Contains(t, stderr.String(), "a, b")
}

func TestEnvShellAliasesAndDefault(t *testing.T) {
	dir := writeFixture(t)
	for _, alias := range []string{"bash", "zsh"} {
		a, stdout, _ := newTestApp(dir, nil)
		require.Equal(t, 0, a.run([]string{"env", "b", "--shell", alias}))
		assert.True(t, strings.HasPrefix(stdout.String(), "export "), "--shell %s", alias)
	}
	a, stdout, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"env", "b"}))
	assert.True(t, strings.HasPrefix(stdout.String(), "export "))
}

func TestEnvQuoting(t *testing.T) {
	assert.Equal(t, `'it'\''s a '\''test'\'''`, quotePosix(`it's a 'test'`))
	assert.Equal(t, `'it\'s \\a'`, quoteFish(`it's \a`))
	assert.Equal(t, `'it''s'`, quotePowershell(`it's`))
}

func TestEnvRejectsNameWithReset(t *testing.T) {
	dir := writeFixture(t)
	a, stdout, _ := newTestApp(dir, nil)
	require.NotEqual(t, 0, a.run([]string{"env", "b", "--reset"}))
	assert.Empty(t, stdout.String())
}
