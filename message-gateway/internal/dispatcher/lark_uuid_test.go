package dispatcher

import "testing"

func TestNormalizeLarkUUID(t *testing.T) {
	got := normalizeLarkUUID("mgw:53ca512e414582ca531e910872b9cb3c:extra")
	if len(got) > 32 {
		t.Fatalf("uuid too long: %d %s", len(got), got)
	}
	for _, r := range got {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			t.Fatalf("uuid contains invalid rune %q in %s", r, got)
		}
	}
}
