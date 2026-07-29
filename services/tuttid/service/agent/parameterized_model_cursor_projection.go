package agent

import (
	"strings"
)

// projectCursorWireModelParameterProfiles builds per-model profiles with the
// audited precedence ACP > exact preset > family preset > parameterized
// current-value evidence. Auto hides Context/reasoning and never invents
// default[fast=true]. Unmatched families only project verified current values.
func projectCursorWireModelParameterProfiles(
	modelOptions []ComposerConfigOptionValue,
	effectiveModel string,
	effectiveParameters map[string]string,
	effectiveSpeed string,
	acpProfiles []ComposerModelParameterProfile,
	rejected map[string]map[string]string,
) []ComposerModelParameterProfile {
	seen := map[string]struct{}{}
	modelIDs := make([]string, 0, len(modelOptions)+1)
	appendModel := func(modelID string) {
		modelID = strings.TrimSpace(modelID)
		if modelID == "" {
			return
		}
		if _, ok := seen[modelID]; ok {
			return
		}
		seen[modelID] = struct{}{}
		modelIDs = append(modelIDs, modelID)
	}
	for _, option := range modelOptions {
		appendModel(option.Value)
	}
	appendModel(effectiveModel)

	parameterized := make([]ComposerModelParameterProfile, 0, len(modelIDs))
	family := make([]ComposerModelParameterProfile, 0, len(modelIDs))
	exact := make([]ComposerModelParameterProfile, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		baseModelID := parameterizedModelBaseID(modelID)
		parsed := parseParameterizedModelID(modelID)
		parameterized = append(parameterized, cursorWireParameterizedCurrentProfile(modelID, baseModelID, parsed))
		if profile, ok := cursorWireExactCapabilityProfile(modelID, baseModelID, parsed); ok {
			exact = append(exact, profile)
		}
		if profile, ok := cursorWireFamilyCapabilityProfile(modelID, baseModelID, parsed); ok {
			family = append(family, profile)
		}
	}

	profiles := resolveComposerModelParameterProfiles(parameterized, family, exact, acpProfiles)
	for index := range profiles {
		profiles[index] = applyCursorWireEffectiveValues(
			profiles[index],
			effectiveModel,
			effectiveParameters,
			effectiveSpeed,
		)
		profiles[index] = applyCursorWireRuntimeRejections(profiles[index], rejected)
	}
	return profiles
}

func cursorWireParameterizedCurrentProfile(
	modelID string,
	baseModelID string,
	parsed parameterizedModelID,
) ComposerModelParameterProfile {
	parameters := make([]ComposerModelParameterCapability, 0, len(parsed.Params))
	if isAutoParameterizedModelID(modelID) {
		return ComposerModelParameterProfile{ModelID: modelID, BaseModelID: baseModelID, Parameters: nil}
	}
	reasoningKey := cursorWireReasoningKey(baseModelID, parsed)
	for _, param := range parsed.Params {
		key := strings.TrimSpace(param.Key)
		if key == "" {
			continue
		}
		switch key {
		case "context":
			parameters = append(parameters, ComposerModelParameterCapability{
				ID: ComposerModelParameterSemanticContext, Semantic: ComposerModelParameterSemanticContext,
				Source:          ComposerModelParameterSourceParameterizedModel,
				PreferenceScope: ComposerModelParameterPreferenceScopeBaseModel,
				Availability:    ComposerModelParameterAvailabilityUnknown,
				CurrentValue:    param.Value,
			})
		case "reasoning", "effort":
			if reasoningKey != "" && key != reasoningKey {
				// Preserve the non-selected wire alias as an opaque unknown so a
				// later rewrite of the active reasoning key cannot drop it.
				parameters = append(parameters, ComposerModelParameterCapability{
					ID: key, Semantic: key,
					Source:          ComposerModelParameterSourceParameterizedModel,
					PreferenceScope: ComposerModelParameterPreferenceScopeBaseModel,
					Availability:    ComposerModelParameterAvailabilityUnknown,
					CurrentValue:    param.Value,
				})
				continue
			}
			parameters = append(parameters, ComposerModelParameterCapability{
				ID: ComposerModelParameterSemanticReasoning, Semantic: ComposerModelParameterSemanticReasoning,
				Source:          ComposerModelParameterSourceParameterizedModel,
				PreferenceScope: ComposerModelParameterPreferenceScopeBaseModel,
				Availability:    ComposerModelParameterAvailabilityUnknown,
				CurrentValue:    param.Value,
			})
		case "fast":
			parameters = append(parameters, ComposerModelParameterCapability{
				ID: ComposerModelParameterSemanticSpeed, Semantic: ComposerModelParameterSemanticSpeed,
				Source:          ComposerModelParameterSourceParameterizedModel,
				PreferenceScope: ComposerModelParameterPreferenceScopeAgentTarget,
				Availability:    ComposerModelParameterAvailabilityUnknown,
				CurrentValue:    cursorWireSpeedFromFastParam(param.Value),
			})
		default:
			parameters = append(parameters, ComposerModelParameterCapability{
				ID: key, Semantic: key,
				Source:          ComposerModelParameterSourceParameterizedModel,
				PreferenceScope: ComposerModelParameterPreferenceScopeBaseModel,
				Availability:    ComposerModelParameterAvailabilityUnknown,
				CurrentValue:    param.Value,
			})
		}
	}
	return ComposerModelParameterProfile{ModelID: modelID, BaseModelID: baseModelID, Parameters: parameters}
}

func cursorWireFamilyCapabilityProfile(
	modelID string,
	baseModelID string,
	parsed parameterizedModelID,
) (ComposerModelParameterProfile, bool) {
	if isAutoParameterizedModelID(modelID) {
		return ComposerModelParameterProfile{}, false
	}
	family, ok := cursorWireFamilyForBase(baseModelID)
	if !ok {
		return ComposerModelParameterProfile{}, false
	}
	parameters := make([]ComposerModelParameterCapability, 0, 3)
	if len(family.contextValues) > 0 {
		current, _ := parsed.Lookup("context")
		parameters = append(parameters, ComposerModelParameterCapability{
			ID: ComposerModelParameterSemanticContext, Semantic: ComposerModelParameterSemanticContext,
			Source:          ComposerModelParameterSourceModelFamilyPreset,
			PreferenceScope: ComposerModelParameterPreferenceScopeBaseModel,
			Availability:    ComposerModelParameterAvailabilitySupported,
			Configurable:    true, CurrentValue: current,
			Options: optionValuesFromStrings(family.contextValues),
		})
	}
	if family.reasoningWireKey != "" && len(family.reasoningValues) > 0 {
		current, _ := parsed.Lookup(family.reasoningWireKey)
		parameters = append(parameters, ComposerModelParameterCapability{
			ID: ComposerModelParameterSemanticReasoning, Semantic: ComposerModelParameterSemanticReasoning,
			Source:          ComposerModelParameterSourceModelFamilyPreset,
			PreferenceScope: ComposerModelParameterPreferenceScopeBaseModel,
			Availability:    ComposerModelParameterAvailabilitySupported,
			Configurable:    true, CurrentValue: current,
			Options: optionValuesFromStrings(family.reasoningValues),
		})
	}
	if len(parameters) == 0 {
		return ComposerModelParameterProfile{}, false
	}
	return ComposerModelParameterProfile{ModelID: modelID, BaseModelID: baseModelID, Parameters: parameters}, true
}

func cursorWireExactCapabilityProfile(
	modelID string,
	baseModelID string,
	parsed parameterizedModelID,
) (ComposerModelParameterProfile, bool) {
	if isAutoParameterizedModelID(modelID) {
		return ComposerModelParameterProfile{}, false
	}
	preset, ok := cursorWireExactPresetForBase(baseModelID)
	if !ok {
		return ComposerModelParameterProfile{}, false
	}
	parameters := make([]ComposerModelParameterCapability, 0, 3)
	if len(preset.contextValues) > 0 {
		current, _ := parsed.Lookup("context")
		parameters = append(parameters, ComposerModelParameterCapability{
			ID: ComposerModelParameterSemanticContext, Semantic: ComposerModelParameterSemanticContext,
			Source:          ComposerModelParameterSourceExactModelPreset,
			PreferenceScope: ComposerModelParameterPreferenceScopeBaseModel,
			Availability:    ComposerModelParameterAvailabilitySupported,
			Configurable:    true, CurrentValue: current,
			Options: optionValuesFromStrings(preset.contextValues),
		})
	}
	if preset.reasoningWireKey != "" && len(preset.reasoningValues) > 0 {
		current, _ := parsed.Lookup(preset.reasoningWireKey)
		parameters = append(parameters, ComposerModelParameterCapability{
			ID: ComposerModelParameterSemanticReasoning, Semantic: ComposerModelParameterSemanticReasoning,
			Source:          ComposerModelParameterSourceExactModelPreset,
			PreferenceScope: ComposerModelParameterPreferenceScopeBaseModel,
			Availability:    ComposerModelParameterAvailabilitySupported,
			Configurable:    true, CurrentValue: current,
			Options: optionValuesFromStrings(preset.reasoningValues),
		})
	}
	if preset.fastSupported {
		current := ""
		if fast, ok := parsed.Lookup("fast"); ok {
			current = cursorWireSpeedFromFastParam(fast)
		}
		parameters = append(parameters, ComposerModelParameterCapability{
			ID: ComposerModelParameterSemanticSpeed, Semantic: ComposerModelParameterSemanticSpeed,
			Source:          ComposerModelParameterSourceExactModelPreset,
			PreferenceScope: ComposerModelParameterPreferenceScopeAgentTarget,
			Availability:    ComposerModelParameterAvailabilitySupported,
			Configurable:    true, CurrentValue: current,
			Options: optionValuesFromStrings([]string{"standard", "fast"}),
		})
	}
	if len(parameters) == 0 {
		return ComposerModelParameterProfile{}, false
	}
	return ComposerModelParameterProfile{ModelID: modelID, BaseModelID: baseModelID, Parameters: parameters}, true
}

func applyCursorWireEffectiveValues(
	profile ComposerModelParameterProfile,
	effectiveModel string,
	effectiveParameters map[string]string,
	effectiveSpeed string,
) ComposerModelParameterProfile {
	if strings.TrimSpace(profile.ModelID) != strings.TrimSpace(effectiveModel) &&
		strings.TrimSpace(effectiveModel) != "" {
		return profile
	}
	parsed := parseParameterizedModelID(profile.ModelID)
	fastSupported := cursorWireFastSupported(profile.ModelID, profile.BaseModelID, false)
	parameters := append([]ComposerModelParameterCapability(nil), profile.Parameters...)
	hasSpeed := false
	for index := range parameters {
		switch parameters[index].Semantic {
		case ComposerModelParameterSemanticContext:
			if value := strings.TrimSpace(effectiveParameters[ComposerModelParameterSemanticContext]); value != "" {
				parameters[index].CurrentValue = value
			}
		case ComposerModelParameterSemanticReasoning:
			if value := strings.TrimSpace(effectiveParameters[ComposerModelParameterSemanticReasoning]); value != "" {
				parameters[index].CurrentValue = value
			}
		case ComposerModelParameterSemanticSpeed:
			hasSpeed = true
			if value := strings.TrimSpace(effectiveSpeed); value != "" {
				parameters[index].CurrentValue = value
			} else if value := strings.TrimSpace(effectiveParameters[ComposerModelParameterSemanticSpeed]); value != "" {
				parameters[index].CurrentValue = value
			} else if fast, ok := parsed.Lookup("fast"); ok {
				parameters[index].CurrentValue = cursorWireSpeedFromFastParam(fast)
			}
			if fastSupported && parameters[index].Availability != ComposerModelParameterAvailabilitySupported {
				parameters[index].Availability = ComposerModelParameterAvailabilitySupported
				parameters[index].Configurable = true
				if len(parameters[index].Options) == 0 {
					parameters[index].Options = optionValuesFromStrings([]string{"standard", "fast"})
					parameters[index].Source = ComposerModelParameterSourceParameterizedModel
				}
			}
		}
	}
	if fastSupported && !hasSpeed && !isAutoParameterizedModelID(profile.ModelID) {
		current := strings.TrimSpace(effectiveSpeed)
		if current == "" {
			current = strings.TrimSpace(effectiveParameters[ComposerModelParameterSemanticSpeed])
		}
		if current == "" {
			if fast, ok := parsed.Lookup("fast"); ok {
				current = cursorWireSpeedFromFastParam(fast)
			}
		}
		parameters = append(parameters, ComposerModelParameterCapability{
			ID: ComposerModelParameterSemanticSpeed, Semantic: ComposerModelParameterSemanticSpeed,
			Source:          ComposerModelParameterSourceParameterizedModel,
			PreferenceScope: ComposerModelParameterPreferenceScopeAgentTarget,
			Availability:    ComposerModelParameterAvailabilitySupported,
			Configurable:    true, CurrentValue: current,
			Options: optionValuesFromStrings([]string{"standard", "fast"}),
		})
	}
	profile.Parameters = parameters
	return profile
}

func applyCursorWireRuntimeRejections(
	profile ComposerModelParameterProfile,
	rejected map[string]map[string]string,
) ComposerModelParameterProfile {
	byParameter, ok := rejected[strings.TrimSpace(profile.BaseModelID)]
	if !ok || len(byParameter) == 0 {
		return profile
	}
	parameters := append([]ComposerModelParameterCapability(nil), profile.Parameters...)
	for index := range parameters {
		rejectedValue := strings.TrimSpace(byParameter[parameters[index].ID])
		if rejectedValue == "" {
			rejectedValue = strings.TrimSpace(byParameter[parameters[index].Semantic])
		}
		if rejectedValue == "" {
			continue
		}
		filtered := parameters[index].Options[:0]
		for _, option := range parameters[index].Options {
			if strings.TrimSpace(option.Value) == rejectedValue {
				continue
			}
			filtered = append(filtered, option)
		}
		parameters[index].Options = filtered
		if len(parameters[index].Options) == 0 {
			parameters[index].Configurable = false
			parameters[index].Availability = ComposerModelParameterAvailabilityUnsupported
		}
	}
	profile.Parameters = parameters
	return profile
}

// rewriteCursorWireModelID applies provider-neutral modelParameters and the
// target-global speed preference onto a Cursor parameterized model id while
// preserving thinking/context/unknown keys. Auto never gains a fabricated
// fast=true suffix.
func rewriteCursorWireModelID(
	modelID string,
	parameters map[string]string,
	speed string,
) string {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" || isAutoParameterizedModelID(modelID) {
		return modelID
	}
	parsed := parseParameterizedModelID(modelID)
	baseModelID := parameterizedModelBaseID(modelID)
	updates := map[string]*string{}
	if value := strings.TrimSpace(parameters[ComposerModelParameterSemanticContext]); value != "" {
		updates["context"] = &value
	}
	if value := strings.TrimSpace(parameters[ComposerModelParameterSemanticReasoning]); value != "" {
		key := cursorWireReasoningKey(baseModelID, parsed)
		if key == "" {
			key = "reasoning"
		}
		updates[key] = &value
	}
	for key, value := range parameters {
		key = strings.TrimSpace(key)
		switch key {
		case "", ComposerModelParameterSemanticContext, ComposerModelParameterSemanticReasoning, ComposerModelParameterSemanticSpeed:
			continue
		}
		selected := strings.TrimSpace(value)
		if selected == "" {
			continue
		}
		updates[key] = &selected
	}
	if cursorWireFastSupported(modelID, baseModelID, false) {
		if value := strings.TrimSpace(parameters[ComposerModelParameterSemanticSpeed]); value != "" {
			fast := cursorWireFastParamFromSpeed(value)
			updates["fast"] = &fast
		} else if strings.TrimSpace(speed) != "" {
			fast := cursorWireFastParamFromSpeed(speed)
			updates["fast"] = &fast
		}
	}
	return parsed.With(updates).Format()
}

func extractCursorWireModelParameters(modelID string) map[string]string {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" || isAutoParameterizedModelID(modelID) {
		return nil
	}
	parsed := parseParameterizedModelID(modelID)
	baseModelID := parameterizedModelBaseID(modelID)
	result := map[string]string{}
	if context, ok := parsed.Lookup("context"); ok {
		result[ComposerModelParameterSemanticContext] = context
	}
	if key := cursorWireReasoningKey(baseModelID, parsed); key != "" {
		if value, ok := parsed.Lookup(key); ok {
			result[ComposerModelParameterSemanticReasoning] = value
		}
	}
	for _, param := range parsed.Params {
		key := strings.TrimSpace(param.Key)
		switch key {
		case "", "context", "reasoning", "effort", "fast":
			continue
		}
		result[key] = param.Value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func extractCursorWireSpeed(modelID string) string {
	parsed := parseParameterizedModelID(strings.TrimSpace(modelID))
	if fast, ok := parsed.Lookup("fast"); ok {
		return cursorWireSpeedFromFastParam(fast)
	}
	return ""
}

func mergeCursorWireModelParameters(
	base map[string]string,
	overlay map[string]string,
) map[string]string {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	result := cloneStringValues(base)
	if result == nil {
		result = map[string]string{}
	}
	for key, value := range overlay {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		result[key] = value
	}
	return result
}
