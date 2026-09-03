package release_test

import (
	"testing"

	release "github.com/gotthboard/gotth-release/pkg/release"
)

func TestCanonicalPublicPackageIsUsable(t *testing.T) {
	t.Parallel()

	identity, err := release.ValidateIdentity("1.2.3", "0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatal(err)
	}
	if identity.Version != "1.2.3" {
		t.Fatalf("Version = %q", identity.Version)
	}
	var _ = release.Config{}
	var _ = release.VerifiedConfig{}
	var _ release.Runner
}
