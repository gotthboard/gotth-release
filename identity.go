// Package release builds normalized, checksummed release archives from exact
// immutable identity and caller-selected files.
package release

import (
	"fmt"

	"github.com/coreos/go-semver/semver"
)

const maximumVersionBytes = 64

// Identity is one canonical semantic version and full lowercase Git commit.
type Identity struct {
	Version string
	Commit  string
}

// ValidateIdentity rejects fabricated, shortened, uppercase, or noncanonical
// release identity.
func ValidateIdentity(version, commit string) (Identity, error) {
	if len(version) == 0 || len(version) > maximumVersionBytes {
		return Identity{}, fmt.Errorf("release version is invalid")
	}
	parsed, err := semver.NewVersion(version)
	if err != nil || parsed.String() != version || !canonicalPreRelease(parsed.PreRelease) {
		return Identity{}, fmt.Errorf("release version is invalid")
	}
	if len(commit) != 40 {
		return Identity{}, fmt.Errorf("release commit is invalid")
	}
	for _, character := range []byte(commit) {
		if character < '0' || character > '9' && character < 'a' || character > 'f' {
			return Identity{}, fmt.Errorf("release commit is invalid")
		}
	}
	return Identity{Version: version, Commit: commit}, nil
}

func canonicalPreRelease(preRelease semver.PreRelease) bool {
	raw := string(preRelease)
	segmentStart := 0
	numeric := true
	for index := 0; index <= len(raw); index++ {
		if index == len(raw) || raw[index] == '.' {
			if numeric && index-segmentStart > 1 && raw[segmentStart] == '0' {
				return false
			}
			segmentStart = index + 1
			numeric = true
			continue
		}
		if raw[index] < '0' || raw[index] > '9' {
			numeric = false
		}
	}
	return true
}
