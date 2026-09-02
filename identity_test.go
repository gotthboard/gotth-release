package release

import "testing"

const testCommit = "0123456789abcdef0123456789abcdef01234567"

func TestValidateIdentity(t *testing.T) {
	identity, err := ValidateIdentity("1.2.3-alpha.4+build", testCommit)
	if err != nil || identity.Version != "1.2.3-alpha.4+build" || identity.Commit != testCommit {
		t.Fatalf("ValidateIdentity() = (%+v, %v)", identity, err)
	}
	for _, values := range [][2]string{
		{"", testCommit}, {"01.2.3", testCommit}, {"1.2.3-alpha.01", testCommit},
		{"1.2.3", "short"}, {"1.2.3", "0123456789ABCDEF0123456789abcdef01234567"},
	} {
		if got, err := ValidateIdentity(values[0], values[1]); err == nil || got != (Identity{}) {
			t.Errorf("ValidateIdentity(%q, %q) = (%+v, %v)", values[0], values[1], got, err)
		}
	}
}
