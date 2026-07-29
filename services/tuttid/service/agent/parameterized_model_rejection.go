package agent

import (
	"fmt"
	"strings"
	"sync"
)

// ModelParameterRejectionError is returned when ACP rejects a rewritten
// parameterized model id. Callers use Evidence for UI rollback copy and the
// daemon caches the exact rejected parameter value for this runtime only.
type ModelParameterRejectionError struct {
	AgentSessionID string
	ModelID        string
	BaseModelID    string
	ParameterID    string
	Value          string
	Evidence       string
	Cause          error
}

func (e *ModelParameterRejectionError) Error() string {
	if e == nil {
		return "model parameter rejected"
	}
	if strings.TrimSpace(e.Evidence) != "" {
		return e.Evidence
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return fmt.Sprintf("model parameter %s=%s rejected for %s", e.ParameterID, e.Value, e.ModelID)
}

func (e *ModelParameterRejectionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

type cursorWireRejectionCache struct {
	mu        sync.Mutex
	bySession map[string]map[string]map[string]string
}

func (c *cursorWireRejectionCache) remember(err *ModelParameterRejectionError) {
	if c == nil || err == nil {
		return
	}
	sessionID := strings.TrimSpace(err.AgentSessionID)
	baseModelID := strings.TrimSpace(err.BaseModelID)
	parameterID := strings.TrimSpace(err.ParameterID)
	value := strings.TrimSpace(err.Value)
	if sessionID == "" || baseModelID == "" || parameterID == "" || value == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.bySession == nil {
		c.bySession = map[string]map[string]map[string]string{}
	}
	byBase := c.bySession[sessionID]
	if byBase == nil {
		byBase = map[string]map[string]string{}
		c.bySession[sessionID] = byBase
	}
	byParameter := byBase[baseModelID]
	if byParameter == nil {
		byParameter = map[string]string{}
		byBase[baseModelID] = byParameter
	}
	byParameter[parameterID] = value
}

func (c *cursorWireRejectionCache) snapshot(agentSessionID string) map[string]map[string]string {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	byBase := c.bySession[strings.TrimSpace(agentSessionID)]
	if len(byBase) == 0 {
		return nil
	}
	result := make(map[string]map[string]string, len(byBase))
	for base, byParameter := range byBase {
		cloned := make(map[string]string, len(byParameter))
		for key, value := range byParameter {
			cloned[key] = value
		}
		result[base] = cloned
	}
	return result
}

func (s *Service) cursorWireRejections() *cursorWireRejectionCache {
	if s == nil {
		return nil
	}
	s.cursorWireRejectionCacheOnce.Do(func() {
		s.cursorWireRejectionCache = &cursorWireRejectionCache{}
	})
	return s.cursorWireRejectionCache
}

func newCursorWireModelParameterRejection(
	agentSessionID string,
	beforeModel string,
	afterModel string,
	parameterID string,
	value string,
	cause error,
) *ModelParameterRejectionError {
	evidence := fmt.Sprintf(
		"ACP rejected parameterized model %q while applying %s=%s (from %q)",
		strings.TrimSpace(afterModel),
		strings.TrimSpace(parameterID),
		strings.TrimSpace(value),
		strings.TrimSpace(beforeModel),
	)
	if cause != nil {
		evidence = evidence + ": " + cause.Error()
	}
	return &ModelParameterRejectionError{
		AgentSessionID: strings.TrimSpace(agentSessionID),
		ModelID:        strings.TrimSpace(afterModel),
		BaseModelID:    parameterizedModelBaseID(afterModel),
		ParameterID:    strings.TrimSpace(parameterID),
		Value:          strings.TrimSpace(value),
		Evidence:       evidence,
		Cause:          cause,
	}
}

func cursorWireRejectionFromUpdateError(
	agentSessionID string,
	beforeModel string,
	patch ComposerSettingsPatch,
	err error,
) *ModelParameterRejectionError {
	if err == nil {
		return nil
	}
	afterModel := strings.TrimSpace(beforeModel)
	if patch.Model != nil {
		afterModel = strings.TrimSpace(*patch.Model)
	}
	parameterID := ComposerModelParameterSemanticReasoning
	value := ""
	if patch.ModelParameters != nil {
		for _, key := range []string{
			ComposerModelParameterSemanticContext,
			ComposerModelParameterSemanticReasoning,
			ComposerModelParameterSemanticSpeed,
		} {
			if selected := patch.ModelParameters[key]; selected != nil && strings.TrimSpace(*selected) != "" {
				parameterID = key
				value = strings.TrimSpace(*selected)
				break
			}
		}
		if value == "" {
			for key, selected := range patch.ModelParameters {
				if selected == nil || strings.TrimSpace(*selected) == "" {
					continue
				}
				parameterID = key
				value = strings.TrimSpace(*selected)
				break
			}
		}
	}
	if value == "" && patch.Speed != nil {
		parameterID = ComposerModelParameterSemanticSpeed
		value = strings.TrimSpace(*patch.Speed)
	}
	if value == "" {
		parameterID = "model"
		value = afterModel
	}
	return newCursorWireModelParameterRejection(agentSessionID, beforeModel, afterModel, parameterID, value, err)
}
