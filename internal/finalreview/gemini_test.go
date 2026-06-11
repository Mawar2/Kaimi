package finalreview

import "testing"

// TestComplianceGenerateConfig_ThinkingHeadroom guards against #192/#229: a
// Gemini 3.x compliance check returns an empty verdict if its thinking tokens
// consume the whole output cap. The config must raise the cap above the old 4096,
// bound the thinking budget so it cannot starve the verdict, and keep JSON output.
func TestComplianceGenerateConfig_ThinkingHeadroom(t *testing.T) {
	cfg := complianceGenerateConfig("verify the draft against the solicitation")

	if cfg.MaxOutputTokens <= 4096 {
		t.Errorf("MaxOutputTokens = %d; must exceed the old 4096 cap that 3.x thinking could consume (#229)", cfg.MaxOutputTokens)
	}
	if cfg.ThinkingConfig == nil || cfg.ThinkingConfig.ThinkingBudget == nil {
		t.Fatal("ThinkingConfig.ThinkingBudget must be set to bound thinking tokens")
	}
	if *cfg.ThinkingConfig.ThinkingBudget >= cfg.MaxOutputTokens {
		t.Errorf("thinking budget %d must leave room for the verdict under MaxOutputTokens %d",
			*cfg.ThinkingConfig.ThinkingBudget, cfg.MaxOutputTokens)
	}
	if cfg.ResponseMIMEType != "application/json" {
		t.Errorf("ResponseMIMEType = %q, want application/json for a structured verdict", cfg.ResponseMIMEType)
	}
	if cfg.SystemInstruction == nil {
		t.Error("SystemInstruction must be preserved")
	}
}
