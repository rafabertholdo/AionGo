package options

import "testing"

func TestStoredValueWinsOverEnvironment(t *testing.T) {
	t.Setenv("MAX_PLAYERS", "50")
	t.Setenv("AION_HTML_WELCOME", "1")
	if got := (Values{"MAX_PLAYERS": "200"}).Get("MAX_PLAYERS", "100"); got != "200" {
		t.Errorf("stored: %q", got)
	}
	if got := (Values{}).Get("MAX_PLAYERS", "100"); got != "50" {
		t.Errorf("environment: %q", got)
	}
	if got := (Values{}).Get("NAME_PATTERN", "x"); got != "x" {
		t.Errorf("fallback: %q", got)
	}
	if !(Values{}).On("AION_HTML_WELCOME", false) {
		t.Error("AION_HTML_WELCOME=1 should be on")
	}
	if (Values{"AION_HTML_WELCOME": "false"}).On("AION_HTML_WELCOME", false) {
		t.Error("a stored false should beat the environment's 1")
	}
	if !(Values{}).On("AION_AUTOCREATE", true) {
		t.Error("an unset switch should take its fallback")
	}
}
