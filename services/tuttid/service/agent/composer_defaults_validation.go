package agent

import (
	"context"
	"fmt"
	"strings"

	preferencesbiz "github.com/tutti-os/tutti/services/tuttid/biz/preferences"
)

func (s *Service) ValidateAgentComposerDefaultsPatch(
	ctx context.Context,
	agentTargetID string,
	patch preferencesbiz.AgentComposerDefaultsPatch,
) error {
	launchInput := CreateSessionInput{
		AgentTargetID: agentTargetID,
	}
	launch, err := s.resolveCreateSessionLaunch(ctx, "", &launchInput)
	if err != nil {
		return err
	}
	settings := ComposerSettings{}
	for field, value := range patch {
		if value == nil {
			continue
		}
		selected := strings.TrimSpace(*value)
		switch field {
		case preferencesbiz.AgentComposerDefaultsFieldModel:
			settings.Model = selected
		case preferencesbiz.AgentComposerDefaultsFieldPermissionModeID:
			settings.PermissionModeID = selected
		case preferencesbiz.AgentComposerDefaultsFieldReasoningEffort:
			settings.ReasoningEffort = selected
		case preferencesbiz.AgentComposerDefaultsFieldSpeed:
			settings.Speed = selected
		}
	}
	options, err := s.GetComposerOptions(ctx, ComposerOptionsInput{
		AgentTargetID:            agentTargetID,
		Provider:                 launch.Provider,
		Settings:                 settings,
		IncludeCapabilityCatalog: boolPointer(false),
	})
	if err != nil {
		return err
	}
	for field, value := range patch {
		if value == nil {
			continue
		}
		selected := strings.TrimSpace(*value)
		switch field {
		case preferencesbiz.AgentComposerDefaultsFieldModel:
			if providerTargetRefKind(launch.ProviderTargetRef) == "agent_extension" {
				observedModels, observed := s.liveComposerModelOptionsForTarget(
					launch.Provider,
					agentTargetID,
				)
				if err := validateComposerDefaultOption(field, selected, observed, observedModels); err != nil {
					return err
				}
			} else if err := s.validateComposerModelForCreate(ctx, launch.Provider, "", "", selected); err != nil {
				return err
			}
		case preferencesbiz.AgentComposerDefaultsFieldPermissionModeID:
			if !options.PermissionConfig.Configurable || !permissionModeConfigHasModeID(options.PermissionConfig, selected) {
				return fmt.Errorf("%w: permission mode is not supported by agent target", ErrInvalidArgument)
			}
		case preferencesbiz.AgentComposerDefaultsFieldReasoningEffort:
			reasoningConfig := composerReasoningConfigForSelectedModel(options)
			if err := validateComposerDefaultOption(field, selected, reasoningConfig.Configurable, reasoningConfig.Options); err != nil {
				return err
			}
		case preferencesbiz.AgentComposerDefaultsFieldSpeed:
			if err := validateComposerDefaultOption(field, selected, options.SpeedConfig.Configurable, options.SpeedConfig.Options); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: unsupported agent composer defaults field", ErrInvalidArgument)
		}
	}
	return nil
}

func (s *Service) ValidateAgentModelParametersPatch(
	ctx context.Context,
	agentTargetID string,
	baseModelID string,
	patch preferencesbiz.AgentModelParametersPatch,
) error {
	hasSelection := false
	for _, value := range patch {
		if value != nil {
			hasSelection = true
			break
		}
	}
	if !hasSelection {
		return nil
	}
	launchInput := CreateSessionInput{AgentTargetID: strings.TrimSpace(agentTargetID)}
	launch, err := s.resolveCreateSessionLaunch(ctx, "", &launchInput)
	if err != nil {
		return err
	}
	options, err := s.GetComposerOptions(ctx, ComposerOptionsInput{
		AgentTargetID: strings.TrimSpace(agentTargetID), Provider: launch.Provider,
		IncludeCapabilityCatalog: boolPointer(false),
	})
	if err != nil {
		return err
	}
	baseModelID = strings.TrimSpace(baseModelID)
	var profile *ComposerModelParameterProfile
	for index := range options.ModelParameterProfiles {
		if strings.TrimSpace(options.ModelParameterProfiles[index].BaseModelID) == baseModelID {
			profile = &options.ModelParameterProfiles[index]
			break
		}
	}
	if profile == nil {
		return fmt.Errorf("%w: base model parameter capabilities are unavailable", ErrInvalidArgument)
	}
	for parameterID, selected := range patch {
		if selected == nil {
			continue
		}
		var capability *ComposerModelParameterCapability
		for index := range profile.Parameters {
			if strings.TrimSpace(profile.Parameters[index].ID) == strings.TrimSpace(parameterID) {
				capability = &profile.Parameters[index]
				break
			}
		}
		if capability == nil || !capability.Configurable ||
			strings.TrimSpace(capability.Availability) != ComposerModelParameterAvailabilitySupported ||
			strings.TrimSpace(capability.PreferenceScope) != ComposerModelParameterPreferenceScopeBaseModel {
			return fmt.Errorf("%w: model parameter %s is not configurable", ErrInvalidArgument, parameterID)
		}
		matched := false
		for _, option := range capability.Options {
			if strings.TrimSpace(option.Value) == strings.TrimSpace(*selected) {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("%w: model parameter %s value is not supported", ErrInvalidArgument, parameterID)
		}
	}
	return nil
}

func validateComposerDefaultOption(
	field string,
	selected string,
	configurable bool,
	options []ComposerConfigOptionValue,
) error {
	if !configurable {
		return fmt.Errorf("%w: %s is not configurable for agent target", ErrInvalidArgument, field)
	}
	if len(options) == 0 {
		return nil
	}
	for _, option := range options {
		if strings.TrimSpace(option.Value) == selected {
			return nil
		}
	}
	return fmt.Errorf("%w: %s value is not supported by agent target", ErrInvalidArgument, field)
}

func (s *Service) validateExtensionComposerSettingsForCreate(
	ctx context.Context,
	workspaceID string,
	cwd string,
	input *CreateSessionInput,
	permissionModeExplicit bool,
) error {
	if input == nil || providerTargetRefKind(input.ProviderTargetRef) != "agent_extension" {
		return nil
	}
	settings := ComposerSettings{
		Model:            strings.TrimSpace(value(input.Model)),
		PermissionModeID: strings.TrimSpace(value(input.PermissionModeID)),
		ReasoningEffort:  strings.TrimSpace(value(input.ReasoningEffort)),
		Speed:            strings.TrimSpace(value(input.Speed)),
	}
	if !permissionModeExplicit {
		// Persisted defaults are fallback preferences, not caller selections.
		// Let Composer Options ignore a stale default and resolve runtime/profile
		// state instead of treating old data as an explicit invalid request.
		settings.PermissionModeID = ""
	}
	options, err := s.GetComposerOptions(ctx, ComposerOptionsInput{
		AgentTargetID:            input.AgentTargetID,
		Provider:                 input.Provider,
		WorkspaceID:              workspaceID,
		Cwd:                      cwd,
		Settings:                 settings,
		IncludeCapabilityCatalog: boolPointer(false),
	})
	if err != nil {
		return err
	}
	if !permissionModeExplicit {
		resolved := strings.TrimSpace(options.EffectiveSettings.PermissionModeID)
		if resolved == "" {
			input.PermissionModeID = nil
		} else {
			input.PermissionModeID = stringPointer(resolved)
		}
	}
	if err := validateExtensionComposerOption(
		preferencesbiz.AgentComposerDefaultsFieldModel,
		settings.Model,
		options.ModelConfig,
	); err != nil {
		return err
	}
	if permissionModeExplicit && settings.PermissionModeID != "" &&
		(!options.PermissionConfig.Configurable ||
			!permissionModeConfigHasModeID(options.PermissionConfig, settings.PermissionModeID)) {
		available := make([]string, 0, len(options.PermissionConfig.Modes))
		for _, mode := range options.PermissionConfig.Modes {
			available = append(available, strings.TrimSpace(mode.ID))
		}
		return &UnsupportedPermissionModeIDError{
			AgentTargetID:              strings.TrimSpace(input.AgentTargetID),
			PermissionModeID:           settings.PermissionModeID,
			AvailablePermissionModeIDs: available,
		}
	}
	reasoningConfig := composerReasoningConfigForSelectedModel(options)
	if err := validateExtensionComposerOption(
		preferencesbiz.AgentComposerDefaultsFieldReasoningEffort,
		settings.ReasoningEffort,
		reasoningConfig,
	); err != nil {
		return err
	}
	return validateExtensionComposerOption(
		preferencesbiz.AgentComposerDefaultsFieldSpeed,
		settings.Speed,
		options.SpeedConfig,
	)
}

func composerReasoningConfigForSelectedModel(options ComposerOptions) ComposerConfigOption {
	model := strings.TrimSpace(options.EffectiveSettings.Model)
	if profile, advertised := options.ReasoningOptionsByModel[model]; advertised {
		return ComposerConfigOption{
			Configurable: len(profile.Options) > 0,
			CurrentValue: strings.TrimSpace(options.EffectiveSettings.ReasoningEffort),
			DefaultValue: strings.TrimSpace(profile.DefaultValue),
			Options:      cloneComposerConfigOptionValues(profile.Options),
		}
	}
	return options.ReasoningConfig
}

func validateExtensionComposerOption(
	field string,
	selected string,
	config ComposerConfigOption,
) error {
	selected = strings.TrimSpace(selected)
	if selected == "" {
		return nil
	}
	if !config.Configurable || len(config.Options) == 0 {
		return fmt.Errorf("%w: %s is not configurable for agent target", ErrInvalidArgument, field)
	}
	for _, option := range config.Options {
		if strings.TrimSpace(option.Value) == selected {
			return nil
		}
	}
	return fmt.Errorf("%w: %s value is not supported by agent target", ErrInvalidArgument, field)
}
