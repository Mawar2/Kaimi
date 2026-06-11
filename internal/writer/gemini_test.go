package writer

import "testing"

// TestSectionGenerateConfig_ThinkingHeadroom guards against #192/#229: the
// Writer's longest sections truncated (2.5-pro) or came back empty (3.x) because
// the model's thinking tokens consumed a low output cap. The config must give
// full-prose headroom and bound the thinking budget so it cannot starve output.
func TestSectionGenerateConfig_ThinkingHeadroom(t *testing.T) {
	cfg := sectionGenerateConfig("stay grounded in the provided sources")

	// Full proposal sections are long prose; the bake-off (#229) target is ~16K
	// so 3.x has room to both think and write. Lock that headroom in.
	if cfg.MaxOutputTokens < 16384 {
		t.Errorf("MaxOutputTokens = %d; full-prose drafting needs >=16384 of headroom (#229)", cfg.MaxOutputTokens)
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
