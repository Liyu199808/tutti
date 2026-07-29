package agent

import (
	"strings"

	"github.com/tutti-os/tutti/packages/agent/daemon/providerregistry"
)

// cursorWireFamily describes one audited Cursor parameterized-model family
// preset. Exact model presets override these when present.
type cursorWireFamily struct {
	match            func(baseModelID string) bool
	reasoningWireKey string
	reasoningValues  []string
	contextValues    []string
	// fastSupportedByFamily is never inferred. Fast is only projected as
	// supported when the exact model id, exact preset, or ACP metadata says so.
}

var cursorWireExactPresets = map[string]cursorWireExactPreset{}

type cursorWireExactPreset struct {
	reasoningWireKey string
	reasoningValues  []string
	contextValues    []string
	fastSupported    bool
}

var cursorWireFamilies = []cursorWireFamily{
	{
		match: func(base string) bool {
			lower := strings.ToLower(base)
			return strings.HasPrefix(lower, "gpt-") || strings.HasPrefix(lower, "openai/gpt-")
		},
		reasoningWireKey: "reasoning",
		reasoningValues:  []string{"none", "low", "medium", "high", "xhigh", "max"},
		contextValues:    []string{"272k", "1m"},
	},
	{
		match: func(base string) bool {
			lower := strings.ToLower(base)
			return strings.HasPrefix(lower, "claude-") || strings.HasPrefix(lower, "anthropic/claude-")
		},
		reasoningWireKey: "effort",
		reasoningValues:  []string{"low", "medium", "high", "xhigh", "max"},
		contextValues:    []string{"300k", "1m"},
	},
	{
		match: func(base string) bool {
			lower := strings.ToLower(base)
			return strings.HasPrefix(lower, "grok-") ||
				strings.HasPrefix(lower, "x-ai/grok-") ||
				strings.HasPrefix(lower, "xai/grok-")
		},
		reasoningWireKey: "effort",
		reasoningValues:  []string{"low", "medium", "high"},
	},
	{
		match: func(base string) bool {
			return strings.HasPrefix(strings.ToLower(base), "composer-")
		},
		// Composer only configures Fast; Fast support is still evidence-gated.
	},
}

func composerUsesCursorWireParameterizedModels(provider string) bool {
	return composerProfileFor(provider).ParameterizedModelCompatibility ==
		providerregistry.ParameterizedModelCompatibilityKindCursorWire
}

func cursorWireFamilyForBase(baseModelID string) (cursorWireFamily, bool) {
	baseModelID = strings.TrimSpace(baseModelID)
	if baseModelID == "" {
		return cursorWireFamily{}, false
	}
	for _, family := range cursorWireFamilies {
		if family.match(baseModelID) {
			return family, true
		}
	}
	return cursorWireFamily{}, false
}

func cursorWireExactPresetForBase(baseModelID string) (cursorWireExactPreset, bool) {
	preset, ok := cursorWireExactPresets[strings.TrimSpace(baseModelID)]
	return preset, ok
}

func cursorWireReasoningKey(baseModelID string, parsed parameterizedModelID) string {
	if preset, ok := cursorWireExactPresetForBase(baseModelID); ok && preset.reasoningWireKey != "" {
		return preset.reasoningWireKey
	}
	if family, ok := cursorWireFamilyForBase(baseModelID); ok && family.reasoningWireKey != "" {
		return family.reasoningWireKey
	}
	if _, ok := parsed.Lookup("reasoning"); ok {
		return "reasoning"
	}
	if _, ok := parsed.Lookup("effort"); ok {
		return "effort"
	}
	return ""
}

func cursorWireFastSupported(modelID string, baseModelID string, acpFastDeclared bool) bool {
	if acpFastDeclared {
		return true
	}
	if preset, ok := cursorWireExactPresetForBase(baseModelID); ok && preset.fastSupported {
		return true
	}
	parsed := parseParameterizedModelID(modelID)
	if _, ok := parsed.Lookup("fast"); ok {
		return true
	}
	return false
}

func cursorWireSpeedFromFastParam(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "true", "1", "yes", "fast":
		return "fast"
	case "false", "0", "no", "standard":
		return "standard"
	default:
		return strings.TrimSpace(value)
	}
}

func cursorWireFastParamFromSpeed(speed string) string {
	switch strings.ToLower(strings.TrimSpace(speed)) {
	case "fast", "true", "1", "yes":
		return "true"
	case "standard", "false", "0", "no", "":
		return "false"
	default:
		return strings.TrimSpace(speed)
	}
}

func optionValuesFromStrings(values []string) []ComposerConfigOptionValue {
	options := make([]ComposerConfigOptionValue, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		options = append(options, ComposerConfigOptionValue{
			ID: value, Label: value, Value: value,
		})
	}
	return options
}
