package mcpserver

import "strings"

// Param mo ta mot tham so tool -> JSON Schema cho tools/list (port cua Param trong McpServer.cs).
type Param struct {
	Name        string
	Type        string // "string" | "integer" | "boolean" | "object" | "array" | "object|array"
	Description string
	Required    bool
	Default     any // nil = khong co "default"; "" / false / 0 van duoc ghi ra
	Items       any
}

func ParamStr(name, description string, required bool, def any) Param {
	return Param{Name: name, Type: "string", Description: description, Required: required, Default: def}
}

func ParamInt(name, description string, required bool, def any) Param {
	return Param{Name: name, Type: "integer", Description: description, Required: required, Default: def}
}

func ParamBool(name, description string, required bool, def any) Param {
	return Param{Name: name, Type: "boolean", Description: description, Required: required, Default: def}
}

func ParamObj(name, description string, required bool) Param {
	return Param{Name: name, Type: "object", Description: description, Required: required}
}

func ParamArr(name, description string, required bool, items any) Param {
	return Param{Name: name, Type: "array", Description: description, Required: required, Items: items}
}

// ParamMatrix = mang cua mang (values 2D trong excel_write/wps_live_write_range).
func ParamMatrix(name, description string, required bool) Param {
	return ParamArr(name, description, required, map[string]any{"type": "array"})
}

// ParamAny nhan nhieu kieu ("object"|"array") - schema ghi "type" la mang.
func ParamAny(name, description string, required bool, types ...string) Param {
	joined := ""
	if len(types) > 0 {
		joined = strings.Join(types, "|")
	}
	return Param{Name: name, Type: joined, Description: description, Required: required}
}

// schemaProperty giu dung thu tu field nhu C#: type, items, description, default.
type schemaProperty struct {
	Type        any    `json:"type,omitempty"`
	Items       any    `json:"items,omitempty"`
	Description string `json:"description,omitempty"`
	Default     any    `json:"default,omitempty"`
}

// Schema dung object JSON Schema. "required" chi xuat hien khi co it nhat mot tham so bat buoc.
func Schema(parameters []Param) *orderedMap {
	properties := newOrderedMap()
	required := []string{}
	for _, parameter := range parameters {
		var typeValue any
		if parameter.Type != "" {
			types := strings.Split(parameter.Type, "|")
			if len(types) == 1 {
				typeValue = types[0]
			} else {
				typeValue = types
			}
		}
		properties.Set(parameter.Name, schemaProperty{
			Type:        typeValue,
			Items:       parameter.Items,
			Description: parameter.Description,
			Default:     parameter.Default,
		})
		if parameter.Required {
			required = append(required, parameter.Name)
		}
	}
	schema := newOrderedMap()
	schema.Set("type", "object")
	schema.Set("properties", properties)
	if len(required) > 0 {
		schema.Set("required", required)
	}
	return schema
}
