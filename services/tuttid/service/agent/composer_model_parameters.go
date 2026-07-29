package agent

import "strings"

const (
	ComposerModelParameterSemanticContext   = "context"
	ComposerModelParameterSemanticReasoning = "reasoning"
	ComposerModelParameterSemanticSpeed     = "speed"

	ComposerModelParameterSourceACP                = "acp"
	ComposerModelParameterSourceExactModelPreset   = "exact-model-preset"
	ComposerModelParameterSourceModelFamilyPreset  = "model-family-preset"
	ComposerModelParameterSourceParameterizedModel = "parameterized-model"

	ComposerModelParameterPreferenceScopeBaseModel   = "baseModel"
	ComposerModelParameterPreferenceScopeAgentTarget = "agentTarget"

	ComposerModelParameterAvailabilitySupported   = "supported"
	ComposerModelParameterAvailabilityUnsupported = "unsupported"
	ComposerModelParameterAvailabilityUnknown     = "unknown"
)

// resolveComposerModelParameterProfiles merges already-decoded capability
// candidates without knowing a provider identity. Resolution is per exact
// model and semantic parameter role. Structured ACP metadata always wins; explicit exact
// model presets beat family presets; a verifiable value parsed from a
// parameterized model id is retained only when no richer capability exists.
func resolveComposerModelParameterProfiles(
	candidates ...[]ComposerModelParameterProfile,
) []ComposerModelParameterProfile {
	type selectedParameter struct {
		capability ComposerModelParameterCapability
		priority   int
	}
	type selectedProfile struct {
		modelID      string
		baseModelID  string
		basePriority int
		parameterIDs []string
		parameters   map[string]selectedParameter
	}
	profiles := map[string]*selectedProfile{}
	modelIDs := []string{}
	for _, group := range candidates {
		for _, candidate := range group {
			modelID := strings.TrimSpace(candidate.ModelID)
			baseModelID := strings.TrimSpace(candidate.BaseModelID)
			if modelID == "" || baseModelID == "" {
				continue
			}
			profile := profiles[modelID]
			if profile == nil {
				profile = &selectedProfile{
					modelID: modelID, baseModelID: baseModelID,
					parameters: map[string]selectedParameter{},
				}
				profiles[modelID] = profile
				modelIDs = append(modelIDs, modelID)
			}
			for _, parameter := range candidate.Parameters {
				parameter.ID = strings.TrimSpace(parameter.ID)
				parameter.Semantic = strings.TrimSpace(parameter.Semantic)
				parameter.Source = strings.TrimSpace(parameter.Source)
				parameter.PreferenceScope = strings.TrimSpace(parameter.PreferenceScope)
				parameter.Availability = strings.TrimSpace(parameter.Availability)
				// Current/default selections are opaque provider values. Validate
				// presence at use sites, but do not normalize their bytes here.
				if parameter.ID == "" || parameter.Semantic == "" || parameter.Source == "" ||
					parameter.PreferenceScope == "" || parameter.Availability == "" {
					continue
				}
				priority := composerModelParameterSourcePriority(parameter.Source)
				selectionKey := parameter.Semantic
				current, found := profile.parameters[selectionKey]
				if found && current.priority >= priority {
					continue
				}
				parameter.Options = cloneComposerConfigOptionValues(parameter.Options)
				profile.parameters[selectionKey] = selectedParameter{capability: parameter, priority: priority}
				if !found {
					profile.parameterIDs = append(profile.parameterIDs, selectionKey)
				}
				if priority > profile.basePriority {
					profile.baseModelID = baseModelID
					profile.basePriority = priority
				}
			}
		}
	}
	result := make([]ComposerModelParameterProfile, 0, len(modelIDs))
	for _, modelID := range modelIDs {
		profile := profiles[modelID]
		parameters := make([]ComposerModelParameterCapability, 0, len(profile.parameterIDs))
		for _, parameterID := range profile.parameterIDs {
			parameters = append(parameters, profile.parameters[parameterID].capability)
		}
		if len(parameters) == 0 {
			continue
		}
		result = append(result, ComposerModelParameterProfile{
			ModelID: profile.modelID, BaseModelID: profile.baseModelID, Parameters: parameters,
		})
	}
	return result
}

func composerModelParameterSourcePriority(source string) int {
	switch strings.TrimSpace(source) {
	case ComposerModelParameterSourceACP:
		return 4
	case ComposerModelParameterSourceExactModelPreset:
		return 3
	case ComposerModelParameterSourceModelFamilyPreset:
		return 2
	case ComposerModelParameterSourceParameterizedModel:
		return 1
	default:
		return 0
	}
}
