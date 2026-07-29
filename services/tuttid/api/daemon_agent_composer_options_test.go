package api

import (
	"testing"

	agentservice "github.com/tutti-os/tutti/services/tuttid/service/agent"
)

// Requested-origin model entries (warm-catalog append of the requested model,
// bootstrap echo) must keep their provenance across the API projection so
// clients can exclude them from catalog testimony; catalog entries omit the
// field entirely (backward-compatible optional).
func TestGeneratedComposerConfigOptionKeepsRequestedProvenance(t *testing.T) {
	generated := generatedComposerConfigOption(agentservice.ComposerConfigOption{
		Configurable:   true,
		CurrentValue:   "default",
		EffectiveValue: "claude-haiku-4-5-20251001",
		Options: []agentservice.ComposerConfigOptionValue{
			{ID: "gpt-5.6-sol", Label: "GPT-5.6 Sol", Value: "gpt-5.6-sol"},
			{ID: "x-ai/grok-4.5", Label: "x-ai/grok-4.5", Value: "x-ai/grok-4.5", Requested: true},
		},
	})
	if len(generated.Options) != 2 {
		t.Fatalf("expected both options, got %d", len(generated.Options))
	}
	if generated.Options[0].Requested != nil {
		t.Fatal("catalog entry must omit the requested field")
	}
	if generated.Options[1].Requested == nil || !*generated.Options[1].Requested {
		t.Fatal("requested-origin entry must project requested=true")
	}
	if generated.CurrentValue == nil || *generated.CurrentValue != "default" {
		t.Fatalf("current value = %#v, want default", generated.CurrentValue)
	}
	if generated.EffectiveValue == nil ||
		*generated.EffectiveValue != "claude-haiku-4-5-20251001" {
		t.Fatalf("effective value = %#v, want resolved Haiku model", generated.EffectiveValue)
	}
}

func TestGeneratedModelParameterProfilesKeepSourceScopeAndCurrentOnlyValues(t *testing.T) {
	generated := generatedAgentProviderModelParameterProfiles([]agentservice.ComposerModelParameterProfile{
		{
			ModelID: "composer-2.5[context=1m,experimental=future]", BaseModelID: "composer-2.5",
			Parameters: []agentservice.ComposerModelParameterCapability{
				{
					ID: "context", Semantic: "context", Source: "acp", PreferenceScope: "baseModel",
					Availability: "supported", Configurable: true, CurrentValue: "1m",
					Options: []agentservice.ComposerConfigOptionValue{
						{ID: "200k", Label: "200K", Value: "200k"},
						{ID: "1m", Label: "1M", Value: "1m"},
					},
				},
				{
					ID: "experimental", Semantic: "experimental", Source: "parameterized-model",
					PreferenceScope: "baseModel", Availability: "unknown", CurrentValue: "future",
				},
			},
		},
	})
	if len(generated) != 1 || len(generated[0].Parameters) != 2 {
		t.Fatalf("generated profiles = %#v", generated)
	}
	contextParameter := generated[0].Parameters[0]
	if contextParameter.Source != "acp" || contextParameter.PreferenceScope != "baseModel" ||
		contextParameter.CurrentValue == nil || *contextParameter.CurrentValue != "1m" || len(contextParameter.Options) != 2 {
		t.Fatalf("context parameter = %#v", contextParameter)
	}
	unknown := generated[0].Parameters[1]
	if unknown.Configurable || unknown.CurrentValue == nil || *unknown.CurrentValue != "future" || len(unknown.Options) != 0 {
		t.Fatalf("current-only unknown parameter = %#v", unknown)
	}
}
