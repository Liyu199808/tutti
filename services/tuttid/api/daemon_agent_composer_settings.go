package api

import (
	"strings"

	tuttigenerated "github.com/tutti-os/tutti/services/tuttid/api/generated"
	agentservice "github.com/tutti-os/tutti/services/tuttid/service/agent"
)

func composerSettingsFromGenerated(settings tuttigenerated.AgentSessionComposerSettings) agentservice.ComposerSettings {
	return agentservice.ComposerSettings{
		Model:            optionalStringValue(settings.Model),
		ModelParameters:  generatedModelParameterValues(settings.ModelParameters),
		PermissionModeID: optionalStringValue(settings.PermissionModeId),
		PlanMode:         settings.PlanMode != nil && *settings.PlanMode,
		BrowserUse:       settings.BrowserUse,
		ReasoningEffort:  optionalStringValue(settings.ReasoningEffort),
		Speed:            optionalStringValue(settings.Speed),
	}
}

func composerSettingsPatchFromGenerated(settings tuttigenerated.AgentSessionComposerSettingsPatch) agentservice.ComposerSettingsPatch {
	return agentservice.ComposerSettingsPatch{
		Model:            settings.Model,
		ModelParameters:  generatedModelParameterPatch(settings.ModelParameters),
		PermissionModeID: settings.PermissionModeId,
		PlanMode:         settings.PlanMode,
		BrowserUse:       settings.BrowserUse,
		ReasoningEffort:  settings.ReasoningEffort,
		Speed:            settings.Speed,
	}
}

func generatedAgentSessionComposerSettings(settings agentservice.ComposerSettings) tuttigenerated.AgentSessionComposerSettings {
	result := tuttigenerated.AgentSessionComposerSettings{
		Model:            optionalStringPointer(strings.TrimSpace(settings.Model)),
		ModelParameters:  optionalModelParameterValues(settings.ModelParameters),
		PermissionModeId: optionalStringPointer(strings.TrimSpace(settings.PermissionModeID)),
		PlanMode:         boolPointer(settings.PlanMode),
		ReasoningEffort:  optionalStringPointer(strings.TrimSpace(settings.ReasoningEffort)),
		Speed:            optionalStringPointer(strings.TrimSpace(settings.Speed)),
	}
	if settings.BrowserUse != nil {
		result.BrowserUse = settings.BrowserUse
	}
	return result
}

func generatedModelParameterValues(values *map[string]string) map[string]string {
	if values == nil {
		return nil
	}
	result := make(map[string]string, len(*values))
	for key, value := range *values {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			result[key] = value
		}
	}
	return result
}

func generatedModelParameterPatch(values *map[string]*string) map[string]*string {
	if values == nil {
		return nil
	}
	result := make(map[string]*string, len(*values))
	for key, value := range *values {
		if strings.TrimSpace(key) == "" {
			continue
		}
		if value == nil {
			result[key] = nil
			continue
		}
		selected := *value
		result[key] = &selected
	}
	return result
}

func optionalModelParameterValues(values map[string]string) *map[string]string {
	result := make(map[string]string, len(values))
	for key, value := range values {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return &result
}
