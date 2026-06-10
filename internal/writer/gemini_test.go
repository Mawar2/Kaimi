package writer

import "testing"

// TestSectionGenerateConfig_ThinkingHeadroom guards against #192: the Writer's
// longest sections truncated mid-sentence because gemini-2.5-pro's thinking
// tokens consumed the 2048 output cap. The config must raise the cap and bound
// the thinking budget so it cannot starve the prose.
func TestSectionGenerateConfig_ThinkingHeadroom(t *testing.T) {
	cfg := sectionGenerateConfig("stay grounded in the provided sources")

	if cfg.MaxOutputTokens <= 2048 {
		t.Errorf("MaxOutputTokens = %d; must exceed the truncating 2048 cap (#192)", cfg.MaxOutputTokens)
	}
	if cfg.ThinkingConfig == nil || cfg.ThinkingConfig.ThinkingBudget == nil {
		t.Fatal("ThinkingConfig.ThinkingBudget must be set to bound thinking tokens")
	}
	if *cfg.ThinkingConfig.ThinkingBudget >= cfg.MaxOutputTokens {
		t.Errorf("thinking budget %d must leave room for output under MaxOutputTokens %d",
			*cfg.ThinkingConfig.ThinkingBudget, cfg.MaxOutputTokens)
	}
	if cfg.SystemInstruction == nil {
		t.Error("SystemInstruction must be preserved")
	}
}
