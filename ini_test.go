package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseINIReadsCoreSection(t *testing.T) {
	ini := parseINI(configA)
	assert.Equal(t, "a@example.com", ini["core"]["account"])
	assert.Equal(t, "proj-a", ini["core"]["project"])
	assert.Equal(t, "europe-west4", ini["compute"]["region"])
}

func TestParseINIEmpty(t *testing.T) {
	assert.Empty(t, parseINI(""))
}

func TestSetINIReplacesExistingKey(t *testing.T) {
	out := setINI(configA, "core", "project", "proj-x")
	assert.Equal(t, "proj-x", parseINI(out)["core"]["project"])
	assert.NotContains(t, out, "proj-a")
	for _, want := range []string{"account = a@example.com", "disable_usage_reporting = True", "[compute]", "region = europe-west4"} {
		assert.Contains(t, out, want)
	}
}

func TestSetINIAddsKeyToExistingSection(t *testing.T) {
	in := "[core]\naccount = a@example.com\n\n[compute]\nregion = europe-west4\n"
	out := setINI(in, "core", "project", "proj-new")
	ini := parseINI(out)
	assert.Equal(t, "proj-new", ini["core"]["project"])
	assert.Empty(t, ini["compute"]["project"])
	assert.Equal(t, "a@example.com", ini["core"]["account"])
}

func TestSetINICreatesSectionInEmptyContent(t *testing.T) {
	out := setINI("", "core", "account", "x@example.com")
	assert.Equal(t, "x@example.com", parseINI(out)["core"]["account"])
	assert.True(t, strings.HasSuffix(out, "\n"))
}

func TestSetINICreatesMissingSection(t *testing.T) {
	out := setINI("[compute]\nregion = europe-west4\n", "core", "project", "p")
	ini := parseINI(out)
	assert.Equal(t, "p", ini["core"]["project"])
	assert.Equal(t, "europe-west4", ini["compute"]["region"])
}
