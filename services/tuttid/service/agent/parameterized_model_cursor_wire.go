package agent

import (
	_ "embed"
	"encoding/json"
	"strings"

	"github.com/tutti-os/tutti/packages/agent/daemon/providerregistry"
)

// cursorWireFamily describes one configured Cursor parameterized-model family.
type cursorWireFamily struct {
	id               string
	prefixes         []string
	reasoningWireKey string
	reasoningValues  []string
	contextValues    []string
	thinkingValues   []string
	fastSupported    bool
}

// cursorWireFamilies is configuration-backed. Cursor ACP commonly reports a
// current wire id but not a selectable parameter range, so these rules provide
// the reviewed family-level fallback. ACP structured capabilities still win.
var cursorWireFamilies = loadCursorWireFamilies()

//go:embed cursor_model_parameters.json
var cursorWireExactPresetsJSON []byte

type cursorWireFamilyJSON struct {
	ID               string   `json:"id"`
	Prefixes         []string `json:"prefixes"`
	ReasoningWireKey string   `json:"reasoningWireKey"`
	ReasoningValues  []string `json:"reasoningValues"`
	ContextValues    []string `json:"contextValues"`
	ThinkingValues   []string `json:"thinkingValues"`
	FastSupported    bool     `json:"fastSupported"`
}

func loadCursorWireFamilies() []cursorWireFamily {
	var entries []cursorWireFamilyJSON
	if err := json.Unmarshal(cursorWireExactPresetsJSON, &entries); err != nil {
		panic("invalid Cursor model parameter family configuration: " + err.Error())
	}
	families := make([]cursorWireFamily, 0, len(entries))
	seenIDs := map[string]struct{}{}
	seenPrefixes := map[string]struct{}{}
	for _, entry := range entries {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			panic("invalid Cursor model parameter family configuration: empty id")
		}
		if _, duplicate := seenIDs[id]; duplicate {
			panic("invalid Cursor model parameter family configuration: duplicate id " + id)
		}
		seenIDs[id] = struct{}{}
		prefixes := normalizedCursorPresetValues(entry.Prefixes)
		if len(prefixes) == 0 {
			panic("invalid Cursor model parameter family configuration: no prefixes for " + id)
		}
		for _, prefix := range prefixes {
			prefix = strings.ToLower(prefix)
			if _, duplicate := seenPrefixes[prefix]; duplicate {
				panic("invalid Cursor model parameter family configuration: duplicate prefix " + prefix)
			}
			seenPrefixes[prefix] = struct{}{}
		}
		families = append(families, cursorWireFamily{
			id:               id,
			prefixes:         prefixes,
			reasoningWireKey: strings.TrimSpace(entry.ReasoningWireKey),
			reasoningValues:  normalizedCursorPresetValues(entry.ReasoningValues),
			contextValues:    normalizedCursorPresetValues(entry.ContextValues),
			thinkingValues:   normalizedCursorPresetValues(entry.ThinkingValues),
			fastSupported:    entry.FastSupported,
		})
	}
	return families
}

func normalizedCursorPresetValues(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, duplicate := seen[value]; duplicate {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
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
	lower := strings.ToLower(baseModelID)
	for _, family := range cursorWireFamilies {
		for _, prefix := range family.prefixes {
			if strings.HasPrefix(lower, strings.ToLower(prefix)) {
				return family, true
			}
		}
	}
	return cursorWireFamily{}, false
}

func cursorWireReasoningKey(baseModelID string, parsed parameterizedModelID) string {
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
	if family, ok := cursorWireFamilyForBase(baseModelID); ok && family.fastSupported {
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
