package eventstream

import (
	"encoding/json"
	"fmt"
	"strings"

	preferencesbiz "github.com/tutti-os/tutti/services/tuttid/biz/preferences"
)

type agentComposerDefaultsPatchRequestedPayload struct {
	AgentTargetID    string                         `json:"agentTargetId"`
	Patch            agentComposerDefaultsPatchBody `json:"patch"`
	ClientMutationID string                         `json:"clientMutationId,omitempty"`
}

type agentComposerDefaultsPatchBody struct {
	Model            *string `json:"model,omitempty"`
	PermissionModeID *string `json:"permissionModeId,omitempty"`
	ReasoningEffort  *string `json:"reasoningEffort,omitempty"`
	Speed            *string `json:"speed,omitempty"`
	ModelParameters  *struct {
		BaseModelID string                                   `json:"baseModelId"`
		Values      preferencesbiz.AgentModelParametersPatch `json:"values"`
	} `json:"modelParameters,omitempty"`
	present map[string]bool
}

func (p *agentComposerDefaultsPatchBody) UnmarshalJSON(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	p.present = make(map[string]bool, len(fields))
	for field, raw := range fields {
		p.present[field] = true
		switch field {
		case preferencesbiz.AgentComposerDefaultsFieldModel:
			if string(raw) != "null" {
				if err := json.Unmarshal(raw, &p.Model); err != nil {
					return err
				}
			}
		case preferencesbiz.AgentComposerDefaultsFieldPermissionModeID:
			if string(raw) != "null" {
				if err := json.Unmarshal(raw, &p.PermissionModeID); err != nil {
					return err
				}
			}
		case preferencesbiz.AgentComposerDefaultsFieldReasoningEffort:
			if string(raw) != "null" {
				if err := json.Unmarshal(raw, &p.ReasoningEffort); err != nil {
					return err
				}
			}
		case preferencesbiz.AgentComposerDefaultsFieldSpeed:
			if string(raw) != "null" {
				if err := json.Unmarshal(raw, &p.Speed); err != nil {
					return err
				}
			}
		case "modelParameters":
			if err := json.Unmarshal(raw, &p.ModelParameters); err != nil {
				return err
			}
		default:
			return fmt.Errorf("patch contains unsupported field %q", field)
		}
	}
	return nil
}

func (p agentComposerDefaultsPatchBody) scalarPatch() preferencesbiz.AgentComposerDefaultsPatch {
	result := preferencesbiz.AgentComposerDefaultsPatch{}
	if p.present[preferencesbiz.AgentComposerDefaultsFieldModel] {
		result[preferencesbiz.AgentComposerDefaultsFieldModel] = p.Model
	}
	if p.present[preferencesbiz.AgentComposerDefaultsFieldPermissionModeID] {
		result[preferencesbiz.AgentComposerDefaultsFieldPermissionModeID] = p.PermissionModeID
	}
	if p.present[preferencesbiz.AgentComposerDefaultsFieldReasoningEffort] {
		result[preferencesbiz.AgentComposerDefaultsFieldReasoningEffort] = p.ReasoningEffort
	}
	if p.present[preferencesbiz.AgentComposerDefaultsFieldSpeed] {
		result[preferencesbiz.AgentComposerDefaultsFieldSpeed] = p.Speed
	}
	return result
}

type agentComposerDefaultsChangedPayload struct {
	AgentTargetID string `json:"agentTargetId"`
}

func validateAgentComposerDefaultsPatchRequestedPayload(payload []byte) error {
	var decoded agentComposerDefaultsPatchRequestedPayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	if strings.TrimSpace(decoded.AgentTargetID) == "" {
		return fmt.Errorf("agentTargetId is required")
	}
	scalarPatch := decoded.Patch.scalarPatch()
	if len(scalarPatch) == 0 && decoded.Patch.ModelParameters == nil {
		return fmt.Errorf("patch is required")
	}
	if len(scalarPatch) > 0 && decoded.Patch.ModelParameters != nil {
		return fmt.Errorf("scalar defaults and model parameters must be patched separately")
	}
	if decoded.Patch.ModelParameters != nil &&
		(strings.TrimSpace(decoded.Patch.ModelParameters.BaseModelID) == "" || len(decoded.Patch.ModelParameters.Values) == 0) {
		return fmt.Errorf("modelParameters requires baseModelId and values")
	}
	return nil
}

func validateAgentComposerDefaultsChangedPayload(payload []byte) error {
	var decoded agentComposerDefaultsChangedPayload
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return fmt.Errorf("decode payload: %w", err)
	}
	if strings.TrimSpace(decoded.AgentTargetID) == "" {
		return fmt.Errorf("agentTargetId is required")
	}
	return nil
}
