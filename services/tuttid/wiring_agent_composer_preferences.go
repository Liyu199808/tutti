package main

import (
	"context"

	preferencesbiz "github.com/tutti-os/tutti/services/tuttid/biz/preferences"
	agentservice "github.com/tutti-os/tutti/services/tuttid/service/agent"
	preferencesservice "github.com/tutti-os/tutti/services/tuttid/service/preferences"
)

func configureAgentComposerPreferences(
	agentSessions *agentservice.Service,
	preferences *preferencesservice.Service,
) {
	agentSessions.AgentComposerDefaultsReader = preferences
	agentSessions.PersistAgentComposerDefaults = func(
		ctx context.Context,
		agentTargetID string,
		patch preferencesbiz.AgentComposerDefaultsPatch,
	) error {
		_, err := preferences.PatchAgentComposerDefaultsForTarget(ctx, preferencesservice.PatchAgentComposerDefaultsForTargetInput{
			AgentTargetID: agentTargetID,
			Patch:         patch,
		})
		return err
	}
	agentSessions.PersistAgentModelParameters = func(
		ctx context.Context,
		agentTargetID string,
		baseModelID string,
		patch preferencesbiz.AgentModelParametersPatch,
	) error {
		_, err := preferences.PatchAgentModelParametersForTarget(ctx, preferencesservice.PatchAgentModelParametersForTargetInput{
			AgentTargetID: agentTargetID,
			BaseModelID:   baseModelID,
			Patch:         patch,
		})
		return err
	}
	preferences.AgentComposerDefaultsValidator = agentSessions
	preferences.AgentModelParametersValidator = agentSessions
}
