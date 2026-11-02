package hybrid

import (
	"testing"

	"github.com/rainchan/webmux/internal/acp"
)

func TestConfigOptionsLabelsModelChoicesByValue(t *testing.T) {
	t.Parallel()
	options := []acp.ConfigOption{
		{
			ID: "model", Name: "Model", Category: "model", Type: acp.ConfigSelect,
			Current: acp.ConfigValue{ID: "anthropic/claude-haiku-4-5"},
			Options: []acp.ConfigSelectOption{
				{Value: "anthropic/claude-haiku-4-5", Name: "Claude Haiku 4.5"},
				{Value: "bedrock/claude-haiku-4-5", Name: "Claude Haiku 4.5"},
			},
		},
		{
			ID: "thought_level", Name: "Thought", Category: "thought_level", Type: acp.ConfigSelect,
			Current: acp.ConfigValue{ID: "high"},
			Options: []acp.ConfigSelectOption{{Value: "high", Name: "High"}},
		},
	}
	byValue := configOptions(options, true)
	if got := byValue[0].Options[1].Name; got != "bedrock/claude-haiku-4-5" {
		t.Errorf("model choice name = %q, want the provider/model id", got)
	}
	if got := byValue[1].Options[0].Name; got != "High" {
		t.Errorf("non-model choice name = %q, want the agent's name", got)
	}
	byName := configOptions(options, false)
	if got := byName[0].Options[1].Name; got != "Claude Haiku 4.5" {
		t.Errorf("unflagged model choice name = %q, want the agent's name", got)
	}
}
