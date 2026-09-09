package winmsi

import (
	"strings"
	"testing"
)

func TestLicenseToRTF(t *testing.T) {
	in := "Copyright {Someone} \\ 2026\nLine two: café"
	out := licenseToRTF(in)

	if !strings.HasPrefix(out, `{\rtf1\ansi\deff0`) {
		t.Errorf("expected RTF header, got:\n%s", out)
	}
	if !strings.HasSuffix(out, "}") {
		t.Errorf("expected RTF document to close with a brace, got:\n%s", out)
	}
	for _, want := range []string{`\{Someone\}`, `\\`, `\par`, `\u233?`} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q (escaped) in output, got:\n%s", want, out)
		}
	}
	if strings.Contains(strings.TrimPrefix(out, `{\rtf1\ansi\deff0`), "café") {
		t.Errorf("expected the non-ASCII 'é' to be RTF-escaped, not passed through raw:\n%s", out)
	}
}
