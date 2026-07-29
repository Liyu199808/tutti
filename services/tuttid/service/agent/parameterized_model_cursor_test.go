package agent

import (
	"errors"
	"slices"
	"testing"
)

func TestParameterizedModelIDRoundTripPreservesUnknownParams(t *testing.T) {
	t.Parallel()
	raw := "claude-sonnet-5[thinking=true,context=300k,effort=high,future=opaque]"
	parsed := parseParameterizedModelID(raw)
	if parsed.Base != "claude-sonnet-5" {
		t.Fatalf("base = %q", parsed.Base)
	}
	high := "xhigh"
	rewritten := parsed.With(map[string]*string{"effort": &high}).Format()
	if rewritten != "claude-sonnet-5[thinking=true,context=300k,effort=xhigh,future=opaque]" {
		t.Fatalf("rewritten = %q", rewritten)
	}
	if got := parseParameterizedModelID(rewritten).Format(); got != rewritten {
		t.Fatalf("round-trip = %q", got)
	}
}

func TestRewriteCursorWireModelIDFamiliesAndAuto(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		model      string
		parameters map[string]string
		speed      string
		want       string
	}{
		{
			name:  "gpt reasoning and context",
			model: "gpt-5.5[context=272k,reasoning=medium,fast=false]",
			parameters: map[string]string{
				"context": "1m", "reasoning": "xhigh",
			},
			speed: "fast",
			want:  "gpt-5.5[context=1m,reasoning=xhigh,fast=true]",
		},
		{
			name:  "claude effort preserves thinking",
			model: "claude-sonnet-5[thinking=true,context=300k,effort=high]",
			parameters: map[string]string{
				"reasoning": "max", "context": "1m",
			},
			want: "claude-sonnet-5[thinking=true,context=1m,effort=max]",
		},
		{
			name:       "grok effort",
			model:      "grok-4.5[effort=low]",
			parameters: map[string]string{"reasoning": "high"},
			want:       "grok-4.5[effort=high]",
		},
		{
			name:  "composer fast only when evidenced",
			model: "composer-2.5[fast=true]",
			speed: "standard",
			want:  "composer-2.5[fast=false]",
		},
		{
			name:  "composer without fast evidence does not invent",
			model: "composer-2.5",
			speed: "fast",
			want:  "composer-2.5",
		},
		{
			name:  "auto never gains fast",
			model: "default[]",
			speed: "fast",
			parameters: map[string]string{
				"context": "1m", "reasoning": "high", "speed": "fast",
			},
			want: "default[]",
		},
		{
			name:  "unknown family keeps current values only",
			model: "gemini-3-pro[context=1m,thinking=true]",
			parameters: map[string]string{
				"context": "200k", "future": "keep-me",
			},
			want: "gemini-3-pro[context=200k,thinking=true,future=keep-me]",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := rewriteCursorWireModelID(tc.model, tc.parameters, tc.speed)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestProjectCursorWireModelParameterProfilesPrecedenceAndFamilies(t *testing.T) {
	t.Parallel()
	modelOptions := []ComposerConfigOptionValue{
		{Value: "default[]", Label: "Auto"},
		{Value: "gpt-5.5[context=272k,reasoning=medium,fast=false]", Label: "gpt-5.5"},
		{Value: "claude-sonnet-5[thinking=true,context=300k,effort=high]", Label: "claude-sonnet-5"},
		{Value: "grok-4.5[effort=low]", Label: "grok-4.5"},
		{Value: "composer-2.5[fast=true]", Label: "composer-2.5"},
		{Value: "gemini-3-pro[context=1m,weird=keep]", Label: "gemini-3-pro"},
	}
	acp := []ComposerModelParameterProfile{{
		ModelID: "gpt-5.5[context=272k,reasoning=medium,fast=false]", BaseModelID: "gpt-5.5",
		Parameters: []ComposerModelParameterCapability{{
			ID: "context", Semantic: "context", Source: ComposerModelParameterSourceACP,
			PreferenceScope: "baseModel", Availability: "supported", Configurable: true, CurrentValue: "272k",
			Options: optionValuesFromStrings([]string{"auto", "272k", "1m"}),
		}},
	}}
	profiles := projectCursorWireModelParameterProfiles(
		modelOptions,
		"gpt-5.5[context=272k,reasoning=medium,fast=false]",
		map[string]string{"context": "272k", "reasoning": "medium"},
		"standard",
		acp,
		nil,
	)
	byModel := map[string]ComposerModelParameterProfile{}
	for _, profile := range profiles {
		byModel[profile.ModelID] = profile
	}
	if _, ok := byModel["default[]"]; ok {
		t.Fatalf("Auto must hide Context/reasoning profiles: %#v", byModel["default[]"])
	}
	gpt := byModel["gpt-5.5[context=272k,reasoning=medium,fast=false]"]
	contextParam := parameterByID(t, gpt, "context")
	if contextParam.Source != ComposerModelParameterSourceACP || len(contextParam.Options) != 3 {
		t.Fatalf("gpt context = %#v, want ACP options", contextParam)
	}
	reasoningParam := parameterByID(t, gpt, "reasoning")
	if !slices.Equal(optionValues(reasoningParam.Options), []string{"none", "low", "medium", "high", "xhigh", "max"}) {
		t.Fatalf("gpt reasoning options = %#v", reasoningParam.Options)
	}
	speedParam := parameterByID(t, gpt, "speed")
	if !speedParam.Configurable || speedParam.CurrentValue != "standard" {
		t.Fatalf("gpt speed = %#v", speedParam)
	}

	claude := byModel["claude-sonnet-5[thinking=true,context=300k,effort=high]"]
	if parameterByID(t, claude, "reasoning").CurrentValue != "high" {
		t.Fatalf("claude reasoning = %#v", claude.Parameters)
	}
	if got := parameterByID(t, claude, "context"); !slices.Equal(optionValues(got.Options), []string{"300k", "1m"}) {
		t.Fatalf("claude context options = %#v", got.Options)
	}
	thinking := parameterByID(t, claude, "thinking")
	if thinking.CurrentValue != "true" || thinking.Configurable {
		t.Fatalf("thinking must stay unknown/current-only: %#v", thinking)
	}

	grok := byModel["grok-4.5[effort=low]"]
	if !slices.Equal(optionValues(parameterByID(t, grok, "reasoning").Options), []string{"low", "medium", "high"}) {
		t.Fatalf("grok reasoning = %#v", grok.Parameters)
	}

	composer := byModel["composer-2.5[fast=true]"]
	if parameterByID(t, composer, "speed").CurrentValue != "fast" {
		t.Fatalf("composer speed = %#v", composer.Parameters)
	}
	for _, parameter := range composer.Parameters {
		if parameter.ID == "context" || parameter.ID == "reasoning" {
			t.Fatalf("composer must not invent context/reasoning: %#v", composer.Parameters)
		}
	}

	gemini := byModel["gemini-3-pro[context=1m,weird=keep]"]
	if parameterByID(t, gemini, "context").Configurable {
		t.Fatalf("unmatched family must stay current-value only: %#v", gemini.Parameters)
	}
	if parameterByID(t, gemini, "weird").CurrentValue != "keep" {
		t.Fatalf("unknown value must be preserved: %#v", gemini.Parameters)
	}
}

func TestCursorWireRuntimeRejectionDisablesExactValue(t *testing.T) {
	t.Parallel()
	profiles := projectCursorWireModelParameterProfiles(
		[]ComposerConfigOptionValue{{Value: "gpt-5.5[context=272k,reasoning=medium,fast=false]"}},
		"gpt-5.5[context=272k,reasoning=medium,fast=false]",
		nil,
		"",
		nil,
		map[string]map[string]string{"gpt-5.5": {"reasoning": "max"}},
	)
	reasoning := parameterByID(t, profiles[0], "reasoning")
	if slices.Contains(optionValues(reasoning.Options), "max") {
		t.Fatalf("rejected value still present: %#v", reasoning.Options)
	}
}

func TestCursorWireRejectionErrorCachesEvidence(t *testing.T) {
	t.Parallel()
	cache := &cursorWireRejectionCache{}
	rejection := newCursorWireModelParameterRejection(
		"session-1",
		"gpt-5.5[reasoning=medium]",
		"gpt-5.5[reasoning=max]",
		"reasoning",
		"max",
		errors.New("account denied"),
	)
	cache.remember(rejection)
	snapshot := cache.snapshot("session-1")
	if snapshot["gpt-5.5"]["reasoning"] != "max" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if rejection.Evidence == "" || rejection.Unwrap() == nil {
		t.Fatalf("rejection = %#v", rejection)
	}
}

func TestApplyCursorWireComposerSettingsPatchRewritesModel(t *testing.T) {
	t.Parallel()
	contextWindow := "1m"
	patch := applyCursorWireComposerSettingsPatch(
		ComposerSettings{Model: "claude-sonnet-5[thinking=true,context=300k,effort=high]"},
		ComposerSettingsPatch{ModelParameters: map[string]*string{"context": &contextWindow, "reasoning": stringPointer("xhigh")}},
	)
	if patch.Model == nil || *patch.Model != "claude-sonnet-5[thinking=true,context=1m,effort=xhigh]" {
		t.Fatalf("model patch = %#v", patch.Model)
	}
	if patch.ModelParameters["context"] == nil || *patch.ModelParameters["context"] != "1m" {
		t.Fatalf("parameters = %#v", patch.ModelParameters)
	}
	if patch.ModelParameters["reasoning"] == nil || *patch.ModelParameters["reasoning"] != "xhigh" {
		t.Fatalf("reasoning parameter = %#v", patch.ModelParameters)
	}
}

func TestGetComposerOptionsProjectsCursorWireProfiles(t *testing.T) {
	t.Parallel()
	runtime := newFakeRuntime()
	runtime.sessions["cursor-1"] = ProviderRuntimeSession{
		ID: "cursor-1", WorkspaceID: "ws-1", Provider: "cursor",
		RuntimeContext: cursorModelRuntimeContext(),
		Settings:       &ComposerSettings{Model: "composer-2.5[fast=true]"},
	}
	service := newIsolatedAgentService(runtime)
	options, err := service.GetComposerOptions(t.Context(), ComposerOptionsInput{
		WorkspaceID: "ws-1", Provider: "cursor",
		Settings: ComposerSettings{Model: "composer-2.5[fast=true]", Speed: "fast"},
	})
	if err != nil {
		t.Fatalf("GetComposerOptions error = %v", err)
	}
	if len(options.ModelParameterProfiles) == 0 {
		t.Fatalf("expected model parameter profiles, got %#v", options)
	}
	composer := findProfile(t, options.ModelParameterProfiles, "composer-2.5[fast=true]")
	if parameterByID(t, composer, "speed").CurrentValue != "fast" {
		t.Fatalf("composer profile = %#v", composer)
	}
	gpt := findProfile(t, options.ModelParameterProfiles, "gpt-5.2[reasoning=medium,fast=false]")
	if !slices.Equal(optionValues(parameterByID(t, gpt, "reasoning").Options), []string{"none", "low", "medium", "high", "xhigh", "max"}) {
		t.Fatalf("gpt profile = %#v", gpt)
	}
}

func parameterByID(t *testing.T, profile ComposerModelParameterProfile, id string) ComposerModelParameterCapability {
	t.Helper()
	for _, parameter := range profile.Parameters {
		if parameter.ID == id {
			return parameter
		}
	}
	t.Fatalf("parameter %q missing from %#v", id, profile.Parameters)
	return ComposerModelParameterCapability{}
}

func findProfile(t *testing.T, profiles []ComposerModelParameterProfile, modelID string) ComposerModelParameterProfile {
	t.Helper()
	for _, profile := range profiles {
		if profile.ModelID == modelID {
			return profile
		}
	}
	t.Fatalf("profile %q missing from %#v", modelID, profiles)
	return ComposerModelParameterProfile{}
}

func optionValues(options []ComposerConfigOptionValue) []string {
	values := make([]string, 0, len(options))
	for _, option := range options {
		values = append(values, option.Value)
	}
	return values
}
