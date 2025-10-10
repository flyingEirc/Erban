package tools

import (
	"encoding/json"
	"strings"

	"github.com/openai/openai-go"
)

type ToolCallbuilder struct {
	Id   string
	Name string
	Args strings.Builder
}

func (t *ToolCallbuilder) Feed(id, name, arg string) {
	if id != "" && t.Id == "" {
		t.Id = id
	}
	if name != "" && t.Name == "" {
		t.Name = name
	}
	if arg != "" {
		t.Args.WriteString(arg)
	}
}

func (t *ToolCallbuilder) Done() (openai.ChatCompletionMessageToolCallParam, error) {
	raw := strings.TrimSpace(t.Args.String())
	if raw == "" {
		raw = "{}"
	}

	var tmp any
	if err := json.Unmarshal([]byte(raw), &tmp); err != nil {
		return openai.ChatCompletionMessageToolCallParam{}, err
	}
	return openai.ChatCompletionMessageToolCallParam{
		ID: t.Id,
		Function: openai.ChatCompletionMessageToolCallFunctionParam{
			Name:      t.Name,
			Arguments: t.Args.String(),
		},
	}, nil
}
