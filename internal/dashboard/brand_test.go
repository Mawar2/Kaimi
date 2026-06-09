package dashboard

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
)

// assertWellFormedXML fails the test if s is not well-formed XML. Inline SVG
// (and the strict attribute-quoted HTML these helpers emit) must parse as XML
// so browsers and template composition never see broken markup.
func assertWellFormedXML(t *testing.T, s string) {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(s))
	for {
		_, err := dec.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("markup is not well-formed XML: %v\nmarkup:\n%s", err, s)
		}
	}
}

func TestMarkVariantsCarryBrandColors(t *testing.T) {
	tests := []struct {
		name        string
		variant     MarkVariant
		wantSubstrs []string
		banSubstrs  []string
	}{
		{
			name:    "primary has cyan sun, blue-to-cyan gradient wave, navy back wave",
			variant: MarkPrimary,
			wantSubstrs: []string{
				`fill="` + BrandCyan + `"`,
				`stop-color="` + BrandBlue + `"`,
				`stop-color="` + BrandCyanDark + `"`,
				`stroke="` + BrandNavy + `"`,
			},
		},
		{
			name:    "reversed swaps to light waves for dark surfaces",
			variant: MarkReversed,
			wantSubstrs: []string{
				`fill="` + BrandCyan + `"`,
				`stroke="#5B9BFF"`,
				`stroke="#FFFFFF"`,
			},
			banSubstrs: []string{BrandNavy},
		},
		{
			name:        "mono navy is single-color",
			variant:     MarkMonoNavy,
			wantSubstrs: []string{`fill="` + BrandNavy + `"`, `stroke="` + BrandNavy + `"`},
			banSubstrs:  []string{BrandCyan, BrandBlue},
		},
		{
			name:        "mono white is single-color reversed",
			variant:     MarkMonoWhite,
			wantSubstrs: []string{`fill="#FFFFFF"`, `stroke="#FFFFFF"`},
			banSubstrs:  []string{BrandCyan, BrandNavy},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(Mark(tt.variant, 46))
			assertWellFormedXML(t, got)
			for _, want := range tt.wantSubstrs {
				if !strings.Contains(got, want) {
					t.Errorf("Mark(%q, 46) missing %q in:\n%s", tt.variant, want, got)
				}
			}
			for _, ban := range tt.banSubstrs {
				if strings.Contains(got, ban) {
					t.Errorf("Mark(%q, 46) must not contain %q in:\n%s", tt.variant, ban, got)
				}
			}
		})
	}
}

func TestMarkTwoWavesAtStandardSizes(t *testing.T) {
	got := string(Mark(MarkPrimary, 46))
	if n := strings.Count(got, "<path"); n != 2 {
		t.Errorf("Mark at 46px should draw two waves, got %d path elements:\n%s", n, got)
	}
	if !strings.Contains(got, `width="46"`) || !strings.Contains(got, `height="46"`) {
		t.Errorf("Mark at 46px should set width/height to 46:\n%s", got)
	}
}

func TestMarkSingleWaveFallbackBelow24px(t *testing.T) {
	got := string(Mark(MarkPrimary, 20))
	assertWellFormedXML(t, got)
	if n := strings.Count(got, "<path"); n != 1 {
		t.Errorf("Mark below 24px should drop the back wave, got %d path elements:\n%s", n, got)
	}
	if strings.Contains(got, "Gradient") {
		t.Errorf("single-wave fallback should use a solid stroke, not a gradient:\n%s", got)
	}
	if !strings.Contains(got, `stroke-width="6.5"`) {
		t.Errorf("single-wave fallback should thicken strokes to 6.5:\n%s", got)
	}
}

func TestMarkClampsBelowMinimumSize(t *testing.T) {
	got := string(Mark(MarkPrimary, 10))
	if !strings.Contains(got, `width="20"`) {
		t.Errorf("Mark should clamp sizes below the 20px brand minimum, got:\n%s", got)
	}
}

func TestMarkUnknownVariantFallsBackToPrimary(t *testing.T) {
	got := Mark(MarkVariant("nonsense"), 46)
	want := Mark(MarkPrimary, 46)
	if got != want {
		t.Errorf("unknown variant should render the primary mark\ngot:  %s\nwant: %s", got, want)
	}
}

func TestFaviconLink(t *testing.T) {
	got := string(FaviconLink())
	assertWellFormedXML(t, got)
	for _, want := range []string{`rel="icon"`, "data:image/svg+xml", "%230A1B3D", "%2322D3EE"} {
		if !strings.Contains(got, want) {
			t.Errorf("FaviconLink() missing %q in:\n%s", want, got)
		}
	}
}

func TestHeaderLockup(t *testing.T) {
	got := string(HeaderLockup())
	assertWellFormedXML(t, got)
	for _, want := range []string{"Kaimi", "THE SEEKER", "<svg"} {
		if !strings.Contains(got, want) {
			t.Errorf("HeaderLockup() missing %q in:\n%s", want, got)
		}
	}
}
