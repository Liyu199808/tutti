package eventstream

import "testing"

func TestAgentComposerDefaultsPayloadValidation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		payload string
		valid   bool
	}{
		{name: "invalid json", payload: `{`, valid: false},
		{name: "missing target", payload: `{"agentTargetId":" ","patch":{"model":"gpt"}}`, valid: false},
		{name: "empty patch", payload: `{"agentTargetId":"local:codex","patch":{}}`, valid: false},
		{name: "unknown field", payload: `{"agentTargetId":"local:codex","patch":{"planMode":"on"}}`, valid: false},
		{name: "model", payload: `{"agentTargetId":"local:codex","patch":{"model":"gpt"}}`, valid: true},
		{name: "permission null", payload: `{"agentTargetId":"local:codex","patch":{"permissionModeId":null}}`, valid: true},
		{name: "reasoning", payload: `{"agentTargetId":"local:codex","patch":{"reasoningEffort":"high"}}`, valid: true},
		{name: "speed", payload: `{"agentTargetId":"local:codex","patch":{"speed":"fast"}}`, valid: true},
		{name: "model parameters", payload: `{"agentTargetId":"local:cursor","patch":{"modelParameters":{"baseModelId":"composer-2.5","values":{"context":"1m","future":null}}}}`, valid: true},
		{name: "model parameters missing base", payload: `{"agentTargetId":"local:cursor","patch":{"modelParameters":{"baseModelId":" ","values":{"context":"1m"}}}}`, valid: false},
		{name: "mixed scalar and model parameters", payload: `{"agentTargetId":"local:cursor","patch":{"speed":"fast","modelParameters":{"baseModelId":"composer-2.5","values":{"context":"1m"}}}}`, valid: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateAgentComposerDefaultsPatchRequestedPayload([]byte(test.payload))
			if (err == nil) != test.valid {
				t.Fatalf("validation error = %v, valid = %v", err, test.valid)
			}
		})
	}
}

func TestAgentComposerDefaultsChangedPayloadValidation(t *testing.T) {
	t.Parallel()
	for payload, valid := range map[string]bool{
		`{`:                                  false,
		`{"agentTargetId":" "}`:              false,
		`{"agentTargetId":"local:opencode"}`: true,
	} {
		err := validateAgentComposerDefaultsChangedPayload([]byte(payload))
		if (err == nil) != valid {
			t.Fatalf("payload %q validation error = %v, valid = %v", payload, err, valid)
		}
	}
}
