package scorer

import "testing"

// TestScoringGenerateConfig_ThinkingHeadroom guards against #192: gemini-2.5-pro
// bills its internal thinking tokens against MaxOutputTokens, so the config must
// (a) raise the cap well above the old 1024 that thinking alone consumed, and
// (b) bound the thinking budget so it cannot starve the JSON output.
func TestScoringGenerateConfig_ThinkingHeadroom(t *testing.T) {
	cfg := scoringGenerateConfig()

	if cfg.MaxOutputTokens <= 1024 {
		t.Errorf("MaxOutputTokens = %d; must exceed the starved 1024 cap (#192)", cfg.MaxOutputTokens)
	}
	if cfg.ThinkingConfig == nil || cfg.ThinkingConfig.ThinkingBudget == nil {
		t.Fatal("ThinkingConfig.ThinkingBudget must be set to bound thinking tokens")
	}
	if *cfg.ThinkingConfig.ThinkingBudget >= cfg.MaxOutputTokens {
		t.Errorf("thinking budget %d must leave room for output under MaxOutputTokens %d",
			*cfg.ThinkingConfig.ThinkingBudget, cfg.MaxOutputTokens)
	}
	if cfg.ResponseSchema == nil {
		t.Error("ResponseSchema must still be set for structured JSON output")
	}
	if cfg.ResponseMIMEType != "application/json" {
		t.Errorf("ResponseMIMEType = %q, want application/json", cfg.ResponseMIMEType)
	}
}
