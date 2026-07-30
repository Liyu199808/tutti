package agent

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	agenthost "github.com/tutti-os/tutti/packages/agent/host"
	"github.com/tutti-os/tutti/services/tuttid/biz/agentprovider"
	preferencesbiz "github.com/tutti-os/tutti/services/tuttid/biz/preferences"
)

func (s *Service) clampReasoningEffortForModel(
	ctx context.Context,
	provider string,
	model string,
	selected string,
) string {
	selected = strings.TrimSpace(selected)
	// Only Codex-derived providers currently treat model-advertised reasoning
	// values as authoritative. OpenCode uses its model catalog for discovery but
	// keeps the static reasoning vocabulary.
	if !composerProviderUsesModelReasoningCatalog(provider) {
		return normalizeReasoningEffortForProvider(provider, selected)
	}
	if strings.TrimSpace(model) == "" && s.ModelCatalog != nil {
		model = composerDefaultModel(ctx, provider, "", s.ModelCatalog)
	}
	catalogOptions, ok := composerModelOptionsFromCatalog(ctx, s.ModelCatalog, provider, "", model)
	if !ok || !catalogOptions.Selection.ReasoningEffortsAdvertised {
		return normalizeReasoningEffortForProvider(provider, selected)
	}
	return resolveAdvertisedReasoningEffort(
		provider,
		selected,
		catalogOptions.Selection.DefaultReasoningEffort,
		catalogOptions.Selection.ReasoningEfforts,
	)
}

func (s *Service) clampReasoningEffortPointerForModel(
	ctx context.Context,
	provider string,
	model string,
	selected *string,
) *string {
	if selected == nil {
		return nil
	}
	clamped := s.clampReasoningEffortForModel(ctx, provider, model, *selected)
	return &clamped
}

func (s *Service) clampReasoningEffortPointerForLaunch(
	ctx context.Context,
	provider string,
	providerTargetRef map[string]any,
	model string,
	selected *string,
) *string {
	if selected == nil {
		return nil
	}
	if providerTargetRefKind(providerTargetRef) == "agent_extension" {
		value := strings.TrimSpace(*selected)
		return &value
	}
	return s.clampReasoningEffortPointerForModel(ctx, provider, model, selected)
}

func (s *Service) clampPersistedSessionReasoningEffortForResume(
	ctx context.Context,
	session PersistedSession,
) PersistedSession {
	if strings.TrimSpace(session.Settings.ReasoningEffort) == "" {
		return session
	}
	if agentprovider.Normalize(session.Provider) == "" {
		session.Settings.ReasoningEffort = strings.TrimSpace(session.Settings.ReasoningEffort)
		return session
	}
	session.Settings.ReasoningEffort = s.clampReasoningEffortForModel(
		ctx,
		session.Provider,
		session.Settings.Model,
		session.Settings.ReasoningEffort,
	)
	return session
}

func (s *Service) UpdateSettings(ctx context.Context, workspaceID string, agentSessionID string, settings ComposerSettingsPatch) (Session, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	agentSessionID = strings.TrimSpace(agentSessionID)
	if workspaceID == "" || agentSessionID == "" {
		return Session{}, ErrInvalidArgument
	}
	release, err := s.acquireSessionSettingsLock(ctx, workspaceID, agentSessionID)
	if err != nil {
		return Session{}, err
	}
	defer release()
	ref := agenthost.SessionRef{WorkspaceID: workspaceID, AgentSessionID: agentSessionID}
	ctx = withServiceHeldSessionLock(ctx, s, ref)
	observed, err := s.ApplicationHost().GetSession(ctx, ref)
	if err != nil {
		return Session{}, err
	}
	provider := strings.TrimSpace(observed.Canonical.Provider)
	runtimeContext := observed.Canonical.InternalRuntimeContext
	currentSettings := composerSettingsFromPayload(observed.Canonical.Settings)
	if observed.Live {
		provider = strings.TrimSpace(observed.Session.Provider)
		runtimeContext = observed.Session.RuntimeContext
		if observed.Session.Settings != nil {
			currentSettings = *observed.Session.Settings
		}
	}
	if settings.Model != nil {
		if err := s.validateSessionModelAgainstRuntimeSnapshot(
			ctx,
			strings.TrimSpace(workspaceID),
			runtimeContext,
			strings.TrimSpace(*settings.Model),
		); err != nil {
			return Session{}, err
		}
	}
	selectedModel := currentSettings.Model
	selectedReasoningEffort := currentSettings.ReasoningEffort
	if settings.Model != nil {
		selectedModel = strings.TrimSpace(*settings.Model)
	}
	if settings.ReasoningEffort != nil {
		selectedReasoningEffort = *settings.ReasoningEffort
	}
	// A live Codex-derived runtime owns the freshest per-model reasoning
	// catalog. Let its adapter resolve active updates; the daemon-side catalog
	// remains the authority for pre-session create/resume only.
	if (settings.Model != nil || settings.ReasoningEffort != nil) &&
		!composerProviderUsesModelReasoningCatalog(provider) {
		clampedReasoningEffort := s.clampReasoningEffortForModel(
			ctx,
			provider,
			selectedModel,
			selectedReasoningEffort,
		)
		if settings.ReasoningEffort != nil || clampedReasoningEffort != selectedReasoningEffort {
			settings.ReasoningEffort = &clampedReasoningEffort
		}
	}
	if settings.Speed != nil {
		normalizedSpeed := strings.TrimSpace(*settings.Speed)
		if !composerUsesCursorWireParameterizedModels(provider) {
			normalizedSpeed = normalizeSpeedForProvider(provider, normalizedSpeed)
		}
		settings.Speed = &normalizedSpeed
	}
	beforeModel := strings.TrimSpace(currentSettings.Model)
	if composerUsesCursorWireParameterizedModels(provider) {
		slog.Info("Cursor model settings update received",
			"event", "agent.cursor_model_switch.service.received",
			"workspace_id", workspaceID,
			"agent_session_id", agentSessionID,
			"current_model", currentSettings.Model,
			"current_speed", currentSettings.Speed,
			"requested_model", strings.TrimSpace(value(settings.Model)),
			"requested_speed", strings.TrimSpace(value(settings.Speed)),
			"requested_parameter_count", len(settings.ModelParameters),
		)
		settings, err = s.applyRememberedCursorWireModelParameters(
			ctx,
			observed.Canonical.AgentTargetID,
			currentSettings,
			settings,
		)
		if err != nil {
			return Session{}, err
		}
		if err := s.validateCursorWireComposerSettingsPatch(
			agentSessionID,
			runtimeContext,
			currentSettings,
			settings,
		); err != nil {
			return Session{}, err
		}
		slog.Info("Cursor model settings update dispatching to host",
			"event", "agent.cursor_model_switch.service.dispatch",
			"workspace_id", workspaceID,
			"agent_session_id", agentSessionID,
			"model", strings.TrimSpace(value(settings.Model)),
			"speed", strings.TrimSpace(value(settings.Speed)),
			"parameter_count", len(settings.ModelParameters),
		)
	}
	requestedSettings := settings
	result, err := s.ApplicationHost().UpdateSettings(ctx, agenthost.UpdateSettingsInput{
		WorkspaceID: workspaceID, AgentSessionID: agentSessionID, Settings: settings,
	})
	if err != nil {
		if composerUsesCursorWireParameterizedModels(provider) {
			slog.Warn("Cursor model settings update failed",
				"event", "agent.cursor_model_switch.service.failed",
				"workspace_id", workspaceID,
				"agent_session_id", agentSessionID,
				"model", strings.TrimSpace(value(requestedSettings.Model)),
				"error", err.Error(),
			)
		}
		if composerUsesCursorWireParameterizedModels(provider) {
			var providerErr *agenthost.ProviderError
			if errors.As(err, &providerErr) {
				rejection := cursorWireRejectionFromUpdateError(
					agentSessionID,
					beforeModel,
					requestedSettings,
					err,
				)
				s.cursorWireRejections().remember(rejection)
				return Session{}, rejection
			}
		}
		return Session{}, err
	}
	if composerUsesCursorWireParameterizedModels(provider) {
		confirmedSettings := composerSettingsFromPayload(result.Canonical.Settings)
		if result.Live && result.Session.Settings != nil {
			confirmedSettings = *result.Session.Settings
		}
		slog.Info("Cursor model settings update confirmed",
			"event", "agent.cursor_model_switch.service.confirmed",
			"workspace_id", workspaceID,
			"agent_session_id", agentSessionID,
			"requested_model", strings.TrimSpace(value(requestedSettings.Model)),
			"confirmed_model", confirmedSettings.Model,
			"confirmed_speed", confirmedSettings.Speed,
		)
		agentTargetID := strings.TrimSpace(result.Canonical.AgentTargetID)
		if requestedSettings.ModelParameters != nil && s.PersistAgentModelParameters != nil {
			confirmedPatch := preferencesbiz.AgentModelParametersPatch{}
			for parameterID, requested := range requestedSettings.ModelParameters {
				if requested == nil {
					confirmedPatch[parameterID] = nil
					continue
				}
				if selected := strings.TrimSpace(confirmedSettings.ModelParameters[parameterID]); selected != "" {
					value := selected
					confirmedPatch[parameterID] = &value
				}
			}
			if len(confirmedPatch) > 0 {
				_ = s.PersistAgentModelParameters(
					ctx,
					agentTargetID,
					parameterizedModelBaseID(confirmedSettings.Model),
					confirmedPatch,
				)
			}
		}
		if requestedSettings.Speed != nil && s.PersistAgentComposerDefaults != nil {
			confirmedSpeed := strings.TrimSpace(confirmedSettings.Speed)
			if confirmedSpeed != "" {
				_ = s.PersistAgentComposerDefaults(
					ctx,
					agentTargetID,
					preferencesbiz.AgentComposerDefaultsPatch{
						preferencesbiz.AgentComposerDefaultsFieldSpeed: &confirmedSpeed,
					},
				)
			}
		}
	}
	return s.projectHostSessionResult(
		ctx,
		result.Canonical,
		result.Session,
		result.Live,
		result.Live,
		true,
	)
}
