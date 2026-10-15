package semver

import (
	"regexp"
	"sync"
)

// mustCompileRegexpOnce returns a regexp that is compiled when needed. The
// function will panic if the regexp is invalid.
func mustCompileRegexpOnce(s string) func() *regexp.Regexp {
	return sync.OnceValue(func() *regexp.Regexp {
		return regexp.MustCompile(s)
	})
}
