package dashboard

import (
	"strings"
	"testing"
)

func TestStyleTagIsAStyleElement(t *testing.T) {
	got := string(StyleTag())
	if !strings.HasPrefix(got, "<style>") || !strings.HasSuffix(got, "</style>") {
		t.Fatalf("StyleTag must emit a single <style> element, got prefix %q / suffix %q",
			got[:min(20, len(got))], got[max(0, len(got)-20):])
	}
}

func TestStyleTagContainsDesignTokens(t *testing.T) {
	got := string(StyleTag())
	// One representative token per token group from kaimi/tokens.css.
	wants := []string{
		"--blue-900: #0A1B3D", // house navy ink
		"--cyan-400: #22D3EE", // Kaimi accent
		"--n-400: #94A3BE",    // navy-tinted neutral
		"--st-human:",         // needs-human amber (the loudest signal)
		"#E8870E",
		"--st-progress:",
		"--rec-nobid:",
		"--fit-strong:",
		"--fit-track:",
		"--urg-crit:",
		"--t-h1:",
		"--s-7: 32px",
		"--r-pill: 999px",
		"--e-4:",
		"--m-slow: 360ms",
		"--ease-spring:",
		`[data-theme="focus"]`, // dark Focus theme ships with the tokens
		"prefers-reduced-motion",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("StyleTag() missing token %q", want)
		}
	}
}

func TestStyleTagContainsComponentClasses(t *testing.T) {
	got := string(StyleTag())
	// One selector per component family from kaimi/ui.css.
	wants := []string{
		".kbadge--human",
		".kbadge--progress .dot",
		".krec--review",
		".kfit",
		".kfit-num",
		".kdead--crit",
		".kbtn--select",
		".kbtn--approve",
		".kbtn--changes",
		".kava",
		".kchip--on",
		".ktag",
		"@keyframes kHumanPulse",
	}
	for _, want := range wants {
		if !strings.Contains(got, want) {
			t.Errorf("StyleTag() missing component rule %q", want)
		}
	}
}

func TestStyleTagLoadsNoExternalAssets(t *testing.T) {
	got := string(StyleTag())
	for _, ban := range []string{"@import", "url(http", "fonts.googleapis"} {
		if strings.Contains(got, ban) {
			t.Errorf("StyleTag must not load external assets, found %q", ban)
		}
	}
}
