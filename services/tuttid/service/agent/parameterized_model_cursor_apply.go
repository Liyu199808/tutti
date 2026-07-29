package agent

import (
	"context"
	"fmt"
	"strings"

	agenthost "github.com/tutti-os/tutti/packages/agent/host"
	"github.com/tutti-os/tutti/services/tuttid/biz/agentprovider"
)

func (s *Service) applyRememberedCursorWireModelParameters(
	ctx context.Context,
	agentTargetID string,
	current ComposerSettings,
	patch agenthost.ComposerSettingsPatch,
) (agenthost.ComposerSettingsPatch, error) {
	if patch.Model == nil || s == nil || s.AgentComposerDefaultsReader == nil {
		return patch, nil
	}
	selectedModel := strings.TrimSpace(*patch.Model)
	selectedBaseModelID := parameterizedModelBaseID(selectedModel)
	if selectedBaseModelID == "" || selectedBaseModelID == parameterizedModelBaseID(current.Model) {
		return patch, nil
	}
	defaults, err := s.AgentComposerDefaultsReader.GetAgentComposerDefaultsForTarget(ctx, strings.TrimSpace(agentTargetID))
	if err != nil {
		return patch, err
	}
	remembered := defaults.ModelParametersByBaseModel[selectedBaseModelID]
	if len(remembered) == 0 {
		return patch, nil
	}
	if patch.ModelParameters == nil {
		patch.ModelParameters = map[string]*string{}
	}
	for parameterID, selected := range remembered {
		parameterID = strings.TrimSpace(parameterID)
		selected = strings.TrimSpace(selected)
		if parameterID == "" || selected == "" {
			continue
		}
		if _, explicitlyPatched := patch.ModelParameters[parameterID]; explicitlyPatched {
			continue
		}
		value := selected
		patch.ModelParameters[parameterID] = &value
	}
	return patch, nil
}

func (s *Service) validateCursorWireComposerSettingsPatch(
	agentSessionID string,
	runtimeContext map[string]any,
	current ComposerSettings,
	patch agenthost.ComposerSettingsPatch,
) error {
	selectedModel := strings.TrimSpace(current.Model)
	if patch.Model != nil {
		selectedModel = strings.TrimSpace(*patch.Model)
	}
	if selectedModel == "" {
		return nil
	}
	profiles := projectCursorWireModelParameterProfiles(
		[]ComposerConfigOptionValue{{ID: selectedModel, Label: selectedModel, Value: selectedModel}},
		selectedModel,
		current.ModelParameters,
		current.Speed,
		extractACPModelParameterProfiles(runtimeContext),
		s.cursorWireRejections().snapshot(agentSessionID),
	)
	var selectedProfile *ComposerModelParameterProfile
	for index := range profiles {
		if strings.TrimSpace(profiles[index].ModelID) == selectedModel {
			selectedProfile = &profiles[index]
			break
		}
	}
	findParameter := func(id, semantic string) *ComposerModelParameterCapability {
		if selectedProfile == nil {
			return nil
		}
		for index := range selectedProfile.Parameters {
			parameter := &selectedProfile.Parameters[index]
			if strings.TrimSpace(parameter.ID) == id ||
				(semantic != "" && strings.TrimSpace(parameter.Semantic) == semantic) {
				return parameter
			}
		}
		return nil
	}
	validateSelection := func(parameterID, semantic, selected string) error {
		parameter := findParameter(parameterID, semantic)
		if parameter == nil || !parameter.Configurable ||
			parameter.Availability != ComposerModelParameterAvailabilitySupported {
			return fmt.Errorf("%w: model parameter %q is not configurable for %q", agenthost.ErrInvalidArgument, parameterID, selectedModel)
		}
		for _, option := range parameter.Options {
			if strings.TrimSpace(option.Value) == selected {
				return nil
			}
		}
		return fmt.Errorf("%w: model parameter %q does not support value %q for %q", agenthost.ErrInvalidArgument, parameterID, selected, selectedModel)
	}
	for parameterID, value := range patch.ModelParameters {
		parameterID = strings.TrimSpace(parameterID)
		if value == nil {
			continue
		}
		selected := strings.TrimSpace(*value)
		if parameterID == "" || selected == "" {
			return fmt.Errorf("%w: model parameter id and value are required", agenthost.ErrInvalidArgument)
		}
		semantic := ""
		switch parameterID {
		case ComposerModelParameterSemanticContext, ComposerModelParameterSemanticReasoning, ComposerModelParameterSemanticSpeed:
			semantic = parameterID
		}
		if semantic == ComposerModelParameterSemanticSpeed {
			return fmt.Errorf("%w: Fast must be updated through the target-global speed setting", agenthost.ErrInvalidArgument)
		}
		if err := validateSelection(parameterID, semantic, selected); err != nil {
			return err
		}
	}
	if patch.Speed != nil {
		selected := strings.TrimSpace(*patch.Speed)
		if selected != "" && selected != strings.TrimSpace(current.Speed) {
			if err := validateSelection(ComposerModelParameterSemanticSpeed, ComposerModelParameterSemanticSpeed, selected); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Service) applyCursorWireModelParameterProfiles(
	input ComposerOptionsInput,
	options ComposerOptions,
) ComposerOptions {
	if model := strings.TrimSpace(options.EffectiveSettings.Model); model != "" {
		extracted := extractCursorWireModelParameters(model)
		options.EffectiveSettings.ModelParameters = mergeCursorWireModelParameters(
			extracted,
			options.EffectiveSettings.ModelParameters,
		)
		if strings.TrimSpace(input.Settings.Speed) == "" {
			options.EffectiveSettings.Speed = extractCursorWireSpeed(model)
		}
		// Best-effort apply remembered/target speed onto the selectable model
		// id for presentation without inventing Auto fast defaults. Rewrite
		// before profile projection so the effective profile has the exact id
		// consumed by the provider-neutral GUI.
		if !isAutoParameterizedModelID(model) {
			rewritten := rewriteCursorWireModelID(
				model,
				options.EffectiveSettings.ModelParameters,
				options.EffectiveSettings.Speed,
			)
			if rewritten != "" && rewritten != model {
				options.EffectiveSettings.Model = rewritten
				options.ModelConfig.CurrentValue = rewritten
			}
		}
	}
	rejected := s.cursorWireRejectionsForComposerOptions(input)
	acpProfiles := extractACPModelParameterProfiles(options.RuntimeContext)
	profiles := projectCursorWireModelParameterProfiles(
		options.ModelConfig.Options,
		options.EffectiveSettings.Model,
		options.EffectiveSettings.ModelParameters,
		options.EffectiveSettings.Speed,
		acpProfiles,
		rejected,
	)
	options.ModelParameterProfiles = profiles
	return options
}

func (s *Service) cursorWireRejectionsForComposerOptions(input ComposerOptionsInput) map[string]map[string]string {
	if s == nil {
		return nil
	}
	cache := s.cursorWireRejections()
	merged := map[string]map[string]string{}
	merge := func(snapshot map[string]map[string]string) {
		for base, byParameter := range snapshot {
			dst := merged[base]
			if dst == nil {
				dst = map[string]string{}
				merged[base] = dst
			}
			for key, value := range byParameter {
				dst[key] = value
			}
		}
	}
	provider := agentprovider.NormalizeOpen(input.Provider)
	agentTargetID := strings.TrimSpace(input.AgentTargetID)
	if s.Runtime == nil || strings.TrimSpace(input.WorkspaceID) == "" {
		return nilOrEmptyRejectionMap(merged)
	}
	for _, session := range s.controller().Sessions(input.WorkspaceID) {
		if agentprovider.NormalizeOpen(session.Provider) != provider {
			continue
		}
		if agentTargetID != "" && strings.TrimSpace(session.AgentTargetID) != agentTargetID {
			continue
		}
		merge(cache.snapshot(session.ID))
	}
	return nilOrEmptyRejectionMap(merged)
}

func nilOrEmptyRejectionMap(values map[string]map[string]string) map[string]map[string]string {
	if len(values) == 0 {
		return nil
	}
	return values
}

// extractACPModelParameterProfiles reads structured ACP per-model parameter
// metadata when a live runtime advertises it under model option
// `modelParameters`. Cursor currently usually omits this and falls through to
// exact/family/current-value sources.
func extractACPModelParameterProfiles(runtimeContext map[string]any) []ComposerModelParameterProfile {
	rawOptions := runtimeContext["configOptions"]
	entries, ok := rawOptions.([]any)
	if !ok {
		if typed, typedOK := rawOptions.([]map[string]any); typedOK {
			entries = make([]any, 0, len(typed))
			for _, entry := range typed {
				entries = append(entries, entry)
			}
		} else {
			return nil
		}
	}
	profiles := make([]ComposerModelParameterProfile, 0)
	for _, entry := range entries {
		option, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if strings.TrimSpace(stringFromAny(option["id"])) != "model" {
			continue
		}
		modelEntries, ok := option["options"].([]any)
		if !ok {
			if typed, typedOK := option["options"].([]map[string]any); typedOK {
				modelEntries = make([]any, 0, len(typed))
				for _, modelEntry := range typed {
					modelEntries = append(modelEntries, modelEntry)
				}
			} else {
				continue
			}
		}
		for _, modelEntry := range modelEntries {
			modelOption, ok := modelEntry.(map[string]any)
			if !ok {
				continue
			}
			modelID := firstNonEmptyString(
				strings.TrimSpace(stringFromAny(modelOption["value"])),
				strings.TrimSpace(stringFromAny(modelOption["id"])),
			)
			if modelID == "" {
				continue
			}
			parameters := decodeACPModelParameterCapabilities(modelOption["modelParameters"])
			if len(parameters) == 0 {
				continue
			}
			profiles = append(profiles, ComposerModelParameterProfile{
				ModelID: modelID, BaseModelID: parameterizedModelBaseID(modelID), Parameters: parameters,
			})
		}
	}
	return profiles
}

func decodeACPModelParameterCapabilities(raw any) []ComposerModelParameterCapability {
	entries, ok := raw.([]any)
	if !ok {
		if typed, typedOK := raw.([]map[string]any); typedOK {
			entries = make([]any, 0, len(typed))
			for _, entry := range typed {
				entries = append(entries, entry)
			}
		} else {
			return nil
		}
	}
	parameters := make([]ComposerModelParameterCapability, 0, len(entries))
	for _, entry := range entries {
		object, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		id := strings.TrimSpace(stringFromAny(object["id"]))
		semantic := strings.TrimSpace(stringFromAny(object["semantic"]))
		if semantic == "" {
			semantic = id
		}
		if id == "" || semantic == "" {
			continue
		}
		availability := strings.TrimSpace(stringFromAny(object["availability"]))
		if availability == "" {
			availability = ComposerModelParameterAvailabilitySupported
		}
		preferenceScope := strings.TrimSpace(stringFromAny(object["preferenceScope"]))
		if preferenceScope == "" {
			if semantic == ComposerModelParameterSemanticSpeed {
				preferenceScope = ComposerModelParameterPreferenceScopeAgentTarget
			} else {
				preferenceScope = ComposerModelParameterPreferenceScopeBaseModel
			}
		}
		configurable := boolFromAny(object["configurable"])
		if object["configurable"] == nil {
			configurable = availability == ComposerModelParameterAvailabilitySupported
		}
		parameters = append(parameters, ComposerModelParameterCapability{
			ID: id, Semantic: semantic,
			Source:          ComposerModelParameterSourceACP,
			PreferenceScope: preferenceScope,
			Availability:    availability,
			Configurable:    configurable,
			CurrentValue:    strings.TrimSpace(stringFromAny(object["currentValue"])),
			DefaultValue:    strings.TrimSpace(stringFromAny(object["defaultValue"])),
			Options:         decodeACPModelParameterOptions(object["options"]),
		})
	}
	return parameters
}

func decodeACPModelParameterOptions(raw any) []ComposerConfigOptionValue {
	entries, ok := raw.([]any)
	if !ok {
		if typed, typedOK := raw.([]map[string]any); typedOK {
			entries = make([]any, 0, len(typed))
			for _, entry := range typed {
				entries = append(entries, entry)
			}
		} else {
			return nil
		}
	}
	options := make([]ComposerConfigOptionValue, 0, len(entries))
	for _, entry := range entries {
		object, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		value := firstNonEmptyString(
			strings.TrimSpace(stringFromAny(object["value"])),
			strings.TrimSpace(stringFromAny(object["id"])),
		)
		if value == "" {
			continue
		}
		label := firstNonEmptyString(
			strings.TrimSpace(stringFromAny(object["name"])),
			strings.TrimSpace(stringFromAny(object["label"])),
			value,
		)
		options = append(options, ComposerConfigOptionValue{
			ID: value, Label: label, Value: value,
		})
	}
	return options
}

func applyCursorWireComposerSettings(settings ComposerSettings) ComposerSettings {
	settings.Model = strings.TrimSpace(settings.Model)
	settings.ModelParameters = cloneStringValues(settings.ModelParameters)
	settings.Speed = strings.TrimSpace(settings.Speed)
	if settings.Model == "" {
		return settings
	}
	fastSupported := cursorWireFastSupported(
		settings.Model,
		parameterizedModelBaseID(settings.Model),
		false,
	)
	if !fastSupported {
		settings.Speed = ""
	}
	extracted := extractCursorWireModelParameters(settings.Model)
	settings.ModelParameters = mergeCursorWireModelParameters(extracted, settings.ModelParameters)
	settings.Model = rewriteCursorWireModelID(settings.Model, settings.ModelParameters, settings.Speed)
	if fastSupported && settings.Speed == "" {
		settings.Speed = extractCursorWireSpeed(settings.Model)
	}
	settings.ModelParameters = extractCursorWireModelParameters(settings.Model)
	return settings
}

func applyCursorWireComposerSettingsPatch(
	current ComposerSettings,
	patch agenthost.ComposerSettingsPatch,
) agenthost.ComposerSettingsPatch {
	next := current
	if patch.Model != nil {
		next.Model = strings.TrimSpace(*patch.Model)
	}
	if parameterizedModelBaseID(next.Model) != parameterizedModelBaseID(current.Model) {
		next.ModelParameters = extractCursorWireModelParameters(next.Model)
	} else {
		next.ModelParameters = cloneStringValues(current.ModelParameters)
	}
	for key, value := range patch.ModelParameters {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if value == nil || strings.TrimSpace(*value) == "" {
			delete(next.ModelParameters, key)
			continue
		}
		if next.ModelParameters == nil {
			next.ModelParameters = map[string]string{}
		}
		next.ModelParameters[key] = strings.TrimSpace(*value)
	}
	if patch.Speed != nil {
		next.Speed = strings.TrimSpace(*patch.Speed)
	}
	rewritten := applyCursorWireComposerSettings(next)
	model := rewritten.Model
	patch.Model = &model
	parameterPatch := map[string]*string{}
	for key, value := range rewritten.ModelParameters {
		selected := value
		parameterPatch[key] = &selected
	}
	// Explicit removals from the incoming patch stay removals.
	for key, value := range patch.ModelParameters {
		if value == nil {
			parameterPatch[key] = nil
		}
	}
	patch.ModelParameters = parameterPatch
	if rewritten.Speed != "" || current.Speed != "" || patch.Speed != nil {
		speed := strings.TrimSpace(rewritten.Speed)
		patch.Speed = &speed
	}
	return patch
}

func boolFromAny(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes":
			return true
		default:
			return false
		}
	default:
		return false
	}
}
