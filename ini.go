package main

import "strings"

// parseINI parses gcloud's flat INI dialect into section -> key -> value.
func parseINI(content string) map[string]map[string]string {
	out := map[string]map[string]string{}
	section := ""
	for _, line := range strings.Split(content, "\n") {
		t := strings.TrimSpace(line)
		switch {
		case t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, ";"):
		case strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]"):
			section = strings.TrimSpace(t[1 : len(t)-1])
		default:
			if k, v, ok := strings.Cut(t, "="); ok && section != "" {
				if out[section] == nil {
					out[section] = map[string]string{}
				}
				out[section][strings.TrimSpace(k)] = strings.TrimSpace(v)
			}
		}
	}
	return out
}

// setINI sets [section] key to value, preserving all other lines and creating
// the section if missing.
func setINI(content, section, key, value string) string {
	lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	header := "[" + section + "]"
	inSection := false
	insertAt := -1
	for i, line := range lines {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
			inSection = t == header
			if inSection {
				insertAt = i + 1
			}
			continue
		}
		if !inSection || t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, ";") {
			continue
		}
		if k, _, ok := strings.Cut(t, "="); ok && strings.TrimSpace(k) == key {
			lines[i] = key + " = " + value
			return strings.Join(lines, "\n") + "\n"
		}
		insertAt = i + 1
	}
	entry := key + " = " + value
	if insertAt < 0 {
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, header, entry)
	} else {
		lines = append(lines[:insertAt], append([]string{entry}, lines[insertAt:]...)...)
	}
	return strings.Join(lines, "\n") + "\n"
}
