package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The token-shaped strings below are FABRICATED. They exist only to give the
// scanner something with the right prefix to match; none of them is, or ever
// was, a real credential. Keep it that way when adding cases.
const (
	fakeAccessToken  = "ya29.MADE-UP-VALUE-FOR-TESTS-0000000000"
	fakeRefreshToken = "1//MADE-UP-VALUE-FOR-TESTS-0000000000"
	fakePrivateKey   = "-----BEGIN PRIVATE KEY-----\nnot-a-key\n-----END PRIVATE KEY-----"
)

func writeLog(t *testing.T, dir, day, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, "logs", day, name)
	mustWrite(t, p, body)
	return p
}

func TestDoctorCleanDirectory(t *testing.T) {
	dir := writeFixture(t)
	writeLog(t, dir, "2026.01.01", "quiet.log", "Command: gcloud config list\nnothing to see\n")

	a, stdout, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"doctor"}))
	assert.Contains(t, stdout.String(), "No credential material found")
}

func TestDoctorNoLogDirectoryIsFine(t *testing.T) {
	// A fresh install has never written a log; that is not a problem to report.
	dir := writeFixture(t)
	a, _, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"doctor"}))
}

func TestDoctorReportsAndNeverPrintsTheValue(t *testing.T) {
	dir := writeFixture(t)
	bad := writeLog(t, dir, "2026.01.02", "auth.log",
		"access_token: '"+fakeAccessToken+"'\nrefresh_token: '"+fakeRefreshToken+"'\n")

	a, stdout, _ := newTestApp(dir, nil)
	require.Equal(t, 1, a.run([]string{"doctor"}), "findings must exit non-zero so CI can use it")

	out := stdout.String()
	assert.Contains(t, out, bad)
	assert.Contains(t, out, "oauth access token")
	assert.Contains(t, out, "oauth refresh token")

	// The whole point: report the file, never the secret.
	assert.NotContains(t, out, fakeAccessToken)
	assert.NotContains(t, out, fakeRefreshToken)
	assert.NotContains(t, out, "ya29.MADE")
	assert.NotContains(t, out, "1//MADE")

	assert.True(t, exists(bad), "a plain report must not delete anything")
}

func TestDoctorFindsPrivateKeys(t *testing.T) {
	dir := writeFixture(t)
	writeLog(t, dir, "2026.01.03", "sa.log", fakePrivateKey)

	a, stdout, _ := newTestApp(dir, nil)
	require.Equal(t, 1, a.run([]string{"doctor"}))
	assert.Contains(t, stdout.String(), "service account private key")
	assert.NotContains(t, stdout.String(), "not-a-key")
}

func TestDoctorFixRemovesOnlyTheAffectedFiles(t *testing.T) {
	dir := writeFixture(t)
	bad := writeLog(t, dir, "2026.01.04", "auth.log", "token: "+fakeAccessToken)
	good := writeLog(t, dir, "2026.01.04", "plain.log", "Command: gcloud version")

	a, stdout, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"doctor", "--fix"}))

	assert.False(t, exists(bad), "affected log should be gone")
	assert.True(t, exists(good), "an unaffected log must survive")
	assert.Contains(t, stdout.String(), "Removed 1 of 1")
}

func TestDoctorIgnoresNonLogFiles(t *testing.T) {
	// Captured ADC files legitimately contain a refresh token. They are gctx's
	// own state, not stray debug output, and must not be reported.
	dir := writeFixture(t)
	mustWrite(t, filepath.Join(dir, "adc-a.json"), `{"refresh_token":"`+fakeRefreshToken+`"}`)

	a, stdout, _ := newTestApp(dir, nil)
	require.Equal(t, 0, a.run([]string{"doctor"}))
	assert.Contains(t, stdout.String(), "No credential material found")
}

func TestDoctorRejectsUnknownFlag(t *testing.T) {
	dir := writeFixture(t)
	a, _, stderr := newTestApp(dir, nil)
	require.Equal(t, 1, a.run([]string{"doctor", "--purge"}))
	assert.Contains(t, strings.ToLower(stderr.String()), "unknown flag")
}
