package agent

import "testing"

func TestResolveComposerModelParameterProfilesUsesAuditedSourcePrecedencePerParameter(t *testing.T) {
	parameterized := []ComposerModelParameterProfile{{
		ModelID: "composer-2.5[context=1m,future=opaque]", BaseModelID: "composer-2.5",
		Parameters: []ComposerModelParameterCapability{
			{ID: "context", Semantic: "context", Source: ComposerModelParameterSourceParameterizedModel, PreferenceScope: "baseModel", Availability: "unknown", CurrentValue: "1m"},
			{ID: "future", Semantic: "future", Source: ComposerModelParameterSourceParameterizedModel, PreferenceScope: "baseModel", Availability: "unknown", CurrentValue: "opaque"},
		},
	}}
	family := []ComposerModelParameterProfile{{
		ModelID: "composer-2.5[context=1m,future=opaque]", BaseModelID: "composer-2.5",
		Parameters: []ComposerModelParameterCapability{{
			ID: "context", Semantic: "context", Source: ComposerModelParameterSourceModelFamilyPreset,
			PreferenceScope: "baseModel", Availability: "supported", Configurable: true,
			Options: []ComposerConfigOptionValue{{ID: "200k", Label: "200K", Value: "200k"}},
		}},
	}}
	exact := []ComposerModelParameterProfile{{
		ModelID: "composer-2.5[context=1m,future=opaque]", BaseModelID: "composer-2.5",
		Parameters: []ComposerModelParameterCapability{{
			ID: "context", Semantic: "context", Source: ComposerModelParameterSourceExactModelPreset,
			PreferenceScope: "baseModel", Availability: "supported", Configurable: true,
			Options: []ComposerConfigOptionValue{{ID: "1m", Label: "1M", Value: "1m"}},
		}},
	}}
	acp := []ComposerModelParameterProfile{{
		ModelID: "composer-2.5[context=1m,future=opaque]", BaseModelID: "composer-2.5",
		Parameters: []ComposerModelParameterCapability{{
			ID: "context-window", Semantic: "context", Source: ComposerModelParameterSourceACP,
			PreferenceScope: "baseModel", Availability: "supported", Configurable: true, CurrentValue: "1m",
			Options: []ComposerConfigOptionValue{{ID: "auto", Label: "Auto", Value: "auto"}, {ID: "1m", Label: "1M", Value: "1m"}},
		}},
	}}

	profiles := resolveComposerModelParameterProfiles(family, parameterized, exact, acp)
	if len(profiles) != 1 || len(profiles[0].Parameters) != 2 {
		t.Fatalf("profiles = %#v", profiles)
	}
	contextParameter := profiles[0].Parameters[0]
	if contextParameter.ID != "context-window" || contextParameter.Source != ComposerModelParameterSourceACP || contextParameter.CurrentValue != "1m" || len(contextParameter.Options) != 2 {
		t.Fatalf("resolved context parameter = %#v", contextParameter)
	}
	unknown := profiles[0].Parameters[1]
	if unknown.ID != "future" || unknown.CurrentValue != "opaque" || unknown.Configurable {
		t.Fatalf("preserved unknown parameter = %#v", unknown)
	}
}
