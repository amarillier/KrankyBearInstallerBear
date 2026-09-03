package packager

import "testing"

func TestParseGitHubURL(t *testing.T) {
	cases := []struct {
		raw       string
		wantOwner string
		wantRepo  string
		wantOK    bool
	}{
		{"https://github.com/amarillier/KrankyBearInstallerBear", "amarillier", "KrankyBearInstallerBear", true},
		{"https://github.com/amarillier/KrankyBearInstallerBear.git", "amarillier", "KrankyBearInstallerBear", true},
		{"git@github.com:amarillier/KrankyBearInstallerBear.git", "amarillier", "KrankyBearInstallerBear", true},
		{"ssh://git@github.com/amarillier/KrankyBearInstallerBear.git", "amarillier", "KrankyBearInstallerBear", true},
		{"https://gitlab.com/amarillier/foo", "", "", false},
		{"not a url", "", "", false},
		{"", "", "", false},
		{"https://github.com/amarillier", "", "", false},
	}

	for _, c := range cases {
		owner, repo, ok := ParseGitHubURL(c.raw)
		if owner != c.wantOwner || repo != c.wantRepo || ok != c.wantOK {
			t.Errorf("ParseGitHubURL(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.raw, owner, repo, ok, c.wantOwner, c.wantRepo, c.wantOK)
		}
	}
}
