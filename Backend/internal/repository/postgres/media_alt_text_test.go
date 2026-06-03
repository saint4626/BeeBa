package postgres

import "testing"

func TestContentImageAltTextDefaultsToContentTitle(t *testing.T) {
	if got := contentImageAltText("", "Kubi+Alt by BlueGua"); got != "Kubi+Alt by BlueGua" {
		t.Fatalf("contentImageAltText blank = %q", got)
	}
	if got := contentImageAltText("   ", "Kubi+Alt by BlueGua"); got != "Kubi+Alt by BlueGua" {
		t.Fatalf("contentImageAltText whitespace = %q", got)
	}
}

func TestContentImageAltTextKeepsExplicitText(t *testing.T) {
	if got := contentImageAltText(" Front view of Kubi ", "Kubi+Alt by BlueGua"); got != "Front view of Kubi" {
		t.Fatalf("contentImageAltText explicit = %q", got)
	}
}
