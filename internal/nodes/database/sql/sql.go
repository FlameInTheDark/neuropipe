// Package sql registers the dynamic SQLite action node.
package sql

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/FlameInTheDark/neuropipe/internal/domain"
	"github.com/FlameInTheDark/neuropipe/internal/nodes"
	"github.com/FlameInTheDark/neuropipe/internal/typespec"
)

type Node = nodes.Implementation

var _ nodes.Node = Node{}

const maxNodeRows = 500

var identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func New() Node {
	return Node{Metadata: definition(), Resolver: resolve, Executor: execute}
}

func Register(registrar nodes.Registrar) error { return registrar.Register(New()) }

func definition() domain.NodeDefinition {
	stringType := domain.TypeSpec{Kind: domain.TypeString}
	intType := domain.TypeSpec{Kind: domain.TypeInt}
	boolType := domain.TypeSpec{Kind: domain.TypeBool}
	rowsType := domain.TypeSpec{Kind: domain.TypeList, Element: &domain.TypeSpec{Kind: domain.TypeMap, Key: &stringType, Value: &domain.TypeSpec{Kind: domain.TypeAny}}}
	columnsType := domain.TypeSpec{Kind: domain.TypeList, Element: &stringType}
	return domain.NodeDefinition{
		Type: "action:sql", Category: "Database", Label: "SQL", Description: "Execute one safely parameterized statement against a registered SQLite database.", Icon: "database", Color: "#22c55e", Mode: domain.NodeImpure, PortContractOwned: true,
		Inputs: []domain.NodePort{{ID: "in", Label: "Exec", Kind: domain.PinExec, Direction: domain.PinInput, Color: "#fafafa", MaxConnections: 1}},
		Outputs: []domain.NodePort{
			{ID: "out", Label: "Then", Kind: domain.PinExec, Direction: domain.PinOutput, Color: "#fafafa", MaxConnections: 1},
			{ID: "columns", Label: "Columns", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataList, Type: &columnsType, Color: "#facc15", MaxConnections: 1},
			{ID: "rows", Label: "Rows", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataList, Type: &rowsType, Color: "#facc15", MaxConnections: 1},
			{ID: "rowsAffected", Label: "Rows affected", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataNumber, Type: &intType, Color: "#86efac", MaxConnections: 1},
			{ID: "lastInsertId", Label: "Last insert ID", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataNumber, Type: &intType, Color: "#86efac", MaxConnections: 1},
			{ID: "truncated", Label: "Truncated", Kind: domain.PinData, Direction: domain.PinOutput, DataType: domain.DataBoolean, Type: &boolType, Color: "#f87171", MaxConnections: 1},
		},
		Fields: []domain.ConfigField{
			{Name: "databaseId", Label: "Database", Kind: "database-select", Required: true},
			{Name: "sql", Label: "SQL", Kind: "sql-editor", Required: true},
			{Name: "parameters", Label: "Parameters", Kind: "sql-parameters"},
		},
		DefaultConfig: map[string]any{"databaseId": "", "sql": "", "parameters": []any{}}, Source: "builtin",
	}
}

func resolve(node domain.FlowNode) (domain.NodeDefinition, error) {
	result := definition()
	parameters, err := configuredParameters(config(node))
	if err != nil {
		return result, err
	}
	inputs := append([]domain.NodePort(nil), result.Inputs...)
	for _, parameter := range parameters {
		typeSpec := parameter.Type
		inputs = append(inputs, domain.NodePort{ID: parameter.ID, Label: parameter.Label, Kind: domain.PinData, Direction: domain.PinInput, DataType: dataType(typeSpec), Type: &typeSpec, Color: color(typeSpec), Required: parameter.Required, MaxConnections: 1})
	}
	result.Inputs = inputs
	return result, nil
}

func execute(ctx context.Context, invocation nodes.Invocation, runtime nodes.Runtime) (nodes.ExecutionResult, error) {
	provider, ok := runtime.(nodes.SQLExecutorProvider)
	if !ok || provider.SQLExecutor() == nil {
		return nodes.ExecutionResult{}, fmt.Errorf("database execution is unavailable")
	}
	databaseID, _ := invocation.Config["databaseId"].(string)
	statement, _ := invocation.Config["sql"].(string)
	parameters, err := configuredParameters(invocation.Config)
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	arguments := make([]domain.SQLArgument, 0, len(parameters))
	for _, parameter := range parameters {
		value, exists := invocation.Inputs[parameter.ID]
		if !exists && parameter.Required {
			return nodes.ExecutionResult{}, fmt.Errorf("SQL parameter %q is required", parameter.Label)
		}
		arguments = append(arguments, domain.SQLArgument{Name: parameter.Name, Value: value})
	}
	result, err := provider.SQLExecutor().ExecuteSQL(ctx, domain.SQLRequest{DatabaseID: strings.TrimSpace(databaseID), SQL: statement, Parameters: arguments, MaxRows: maxNodeRows})
	if err != nil {
		return nodes.ExecutionResult{}, err
	}
	lastInsertID := int64(0)
	if result.LastInsertID != nil {
		lastInsertID = *result.LastInsertID
	}
	return nodes.ExecutionResult{Outputs: map[string]any{"columns": result.Columns, "rows": result.Rows, "rowsAffected": result.RowsAffected, "lastInsertId": lastInsertID, "truncated": result.Truncated}, Ports: []string{"out"}}, nil
}

func configuredParameters(values map[string]any) ([]domain.SQLParameter, error) {
	raw, exists := values["parameters"]
	if !exists || raw == nil {
		return []domain.SQLParameter{}, nil
	}
	items, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("SQL parameters must be a list")
	}
	parameters := make([]domain.SQLParameter, 0, len(items))
	ids, names := make(map[string]struct{}), make(map[string]struct{})
	for index, item := range items {
		encoded, err := json.Marshal(item)
		if err != nil {
			return nil, fmt.Errorf("SQL parameter %d: %w", index+1, err)
		}
		var parameter domain.SQLParameter
		if err := json.Unmarshal(encoded, &parameter); err != nil {
			return nil, fmt.Errorf("SQL parameter %d: %w", index+1, err)
		}
		parameter.ID = strings.TrimSpace(parameter.ID)
		parameter.Name = strings.TrimLeft(strings.TrimSpace(parameter.Name), ":@$")
		parameter.Label = strings.TrimSpace(parameter.Label)
		if !identifier.MatchString(parameter.ID) {
			return nil, fmt.Errorf("SQL parameter %d has an invalid pin ID", index+1)
		}
		if !identifier.MatchString(parameter.Name) {
			return nil, fmt.Errorf("SQL parameter %d has an invalid name", index+1)
		}
		if _, duplicate := ids[parameter.ID]; duplicate {
			return nil, fmt.Errorf("SQL parameters contain duplicate pin ID %q", parameter.ID)
		}
		if _, duplicate := names[parameter.Name]; duplicate {
			return nil, fmt.Errorf("SQL parameters contain duplicate name %q", parameter.Name)
		}
		if err := typespec.ValidateSpec(parameter.Type); err != nil {
			return nil, fmt.Errorf("SQL parameter %q type: %w", parameter.Name, err)
		}
		if parameter.Label == "" {
			parameter.Label = parameter.Name
		}
		ids[parameter.ID], names[parameter.Name] = struct{}{}, struct{}{}
		parameters = append(parameters, parameter)
	}
	return parameters, nil
}

func config(node domain.FlowNode) map[string]any {
	if config, ok := node.Data["config"].(map[string]any); ok {
		return config
	}
	return map[string]any{}
}

func dataType(spec domain.TypeSpec) domain.DataType {
	switch spec.Kind {
	case domain.TypeString, domain.TypeBytes:
		return domain.DataText
	case domain.TypeInt, domain.TypeFloat:
		return domain.DataNumber
	case domain.TypeBool:
		return domain.DataBoolean
	case domain.TypeList:
		return domain.DataList
	case domain.TypeMap, domain.TypeRecord:
		return domain.DataObject
	default:
		return domain.DataAny
	}
}

func color(spec domain.TypeSpec) string {
	switch dataType(spec) {
	case domain.DataText:
		return "#e879f9"
	case domain.DataNumber:
		return "#86efac"
	case domain.DataBoolean:
		return "#f87171"
	case domain.DataObject:
		return "#60a5fa"
	case domain.DataList:
		return "#facc15"
	default:
		return "#a1a1aa"
	}
}
