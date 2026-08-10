package pipeline

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
)

func TestFunctionToolInputsUseStableNamesAndStrictTypes(t *testing.T) {
	function := domain.CustomFunction{ID: "a5be12e0-0000-0000-0000-000000000042", Name: "Get city forecast", Description: "Get the current forecast for one city.", Kind: domain.FunctionTool, Inputs: []domain.FunctionPin{
		{ID: "city", Name: "City", Description: "The city and country to look up, for example Yekaterinburg, RU.", DataType: domain.DataText, Required: true},
		{ID: "count", Name: "Count", Description: "The number of forecast days to return.", Type: &domain.TypeSpec{Kind: domain.TypeInt}, Required: true},
	}, Outputs: []domain.FunctionPin{{ID: "forecast", Name: "Forecast", Description: "A concise forecast for the requested city.", DataType: domain.DataText, Required: true}}}
	tests := []struct {
		name    string
		values  map[string]any
		wantErr string
	}{
		{name: "valid", values: map[string]any{"city": "Yekaterinburg", "count": 2}},
		{name: "wrong type", values: map[string]any{"city": "Yekaterinburg", "count": "2"}, wantErr: "Count"},
		{name: "unknown input", values: map[string]any{"city": "Yekaterinburg", "count": 2, "extra": true}, wantErr: "unknown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := functionCallInputs(function, test.values)
			if test.wantErr == "" && err != nil {
				t.Fatalf("functionCallInputs() error = %v", err)
			}
			if test.wantErr != "" && (err == nil || !strings.Contains(err.Error(), test.wantErr)) {
				t.Fatalf("functionCallInputs() error = %v, want %q", err, test.wantErr)
			}
		})
	}

	tool, err := makeConnectedTool(function)
	if err != nil {
		t.Fatal(err)
	}
	properties := tool.definition.InputSchema["properties"].(map[string]any)
	if properties["city"].(map[string]any)["type"] != "string" || properties["count"].(map[string]any)["type"] != "integer" {
		t.Fatalf("tool schema properties = %#v", properties)
	}
	if properties["city"].(map[string]any)["description"] != function.Inputs[0].Description || !strings.HasPrefix(tool.definition.Name, "tool_get_city_forecast_") || !strings.Contains(tool.definition.Description, "forecast") {
		t.Fatalf("tool definition = %#v", tool.definition)
	}
}

func TestToolArgumentsDecodeJSONIntoTheirDeclaredGoTypes(t *testing.T) {
	itemType := domain.TypeSpec{Kind: domain.TypeInt}
	function := domain.CustomFunction{Kind: domain.FunctionTool, Inputs: []domain.FunctionPin{
		{ID: "count", Name: "Count", Description: "Number of items.", Type: &itemType},
		{ID: "content", Name: "Content", Description: "Raw binary content encoded as Base64.", Type: &domain.TypeSpec{Kind: domain.TypeBytes}},
		{ID: "items", Name: "Items", Description: "Integer item identifiers.", Type: &domain.TypeSpec{Kind: domain.TypeList, Element: &itemType}},
	}}
	decoded, err := decodeToolArguments(function, map[string]any{
		"count":   json.Number("3"),
		"content": base64.StdEncoding.EncodeToString([]byte{0xff, 0x00}),
		"items":   []any{json.Number("1"), json.Number("2")},
	})
	if err != nil {
		t.Fatalf("decodeToolArguments() error = %v", err)
	}
	if count, ok := decoded["count"].(int64); !ok || count != 3 {
		t.Fatalf("decoded count = %#v", decoded["count"])
	}
	if content, ok := decoded["content"].([]byte); !ok || string(content) != string([]byte{0xff, 0x00}) {
		t.Fatalf("decoded content = %#v", decoded["content"])
	}
	items, ok := decoded["items"].([]any)
	if !ok || len(items) != 2 || items[0] != int64(1) || items[1] != int64(2) {
		t.Fatalf("decoded items = %#v", decoded["items"])
	}
}

func TestToolFunctionContractRejectsUngroundedTypesAndMissingGuidance(t *testing.T) {
	base := domain.CustomFunction{ID: "tool", Name: "Search", Description: "Search a known local index.", Kind: domain.FunctionTool, Inputs: []domain.FunctionPin{{ID: "query", Name: "Query", Description: "Search phrase.", DataType: domain.DataAny}}, Outputs: []domain.FunctionPin{{ID: "result", Name: "Result", Description: "Matching result.", DataType: domain.DataText}}}
	if err := ValidateToolFunction(base); err == nil || !strings.Contains(err.Error(), "concrete") {
		t.Fatalf("ValidateToolFunction() error = %v, want concrete type error", err)
	}
	base.Inputs[0].DataType = domain.DataText
	base.Outputs[0].Description = ""
	if err := ValidateToolFunction(base); err == nil || !strings.Contains(err.Error(), "guidance") {
		t.Fatalf("ValidateToolFunction() error = %v, want guidance error", err)
	}
}

func TestConfiguredToolTurnsRequiresTypedPositiveInteger(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		want    int
		wantErr bool
	}{
		{name: "default", config: map[string]any{}, want: 8},
		{name: "typed number", config: map[string]any{"maxTurns": 3.0}, want: 3},
		{name: "text is not coerced", config: map[string]any{"maxTurns": "3"}, wantErr: true},
		{name: "fraction", config: map[string]any{"maxTurns": 1.5}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := configuredToolTurns(test.config)
			if test.wantErr {
				if err == nil {
					t.Fatal("configuredToolTurns() error = nil")
				}
				return
			}
			if err != nil || got != test.want {
				t.Fatalf("configuredToolTurns() = %d, %v; want %d, nil", got, err, test.want)
			}
		})
	}
}
