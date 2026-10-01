package game

import (
	"regexp"
	"strings"
	"unicode"
)

// namePattern is gameserver.character.name.pattern, matched against the whole name.
type namePattern struct{ re *regexp.Regexp }

func compileNamePattern(pattern string) (*namePattern, error) {
	if pattern == "" {
		pattern = "[a-zA-Z]{2,16}"
	}
	re, err := regexp.Compile("^(?:" + pattern + ")$")
	return &namePattern{re}, err
}

func (p *namePattern) valid(name string) bool { return p.re.MatchString(name) }

// convertName is how AL-Game stores a name: first letter upper case, the rest lower case.
func convertName(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(strings.ToLower(name))
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}
