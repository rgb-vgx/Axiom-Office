package mcpserver

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// argError la loi tham so cua tool. C# nem ArgumentException tu ToolArgs va McpServer bat lai
// thanh {"ok":false,"error":...} voi isError=true - o day dung panic/recover de ma port giu
// dung hinh dang lambda ngan gon cua ban C#.
type argError struct{ message string }

func (e argError) Error() string { return e.message }

func fail(format string, values ...any) {
	panic(argError{message: fmt.Sprintf(format, values...)})
}

// ToolArgs doc arguments cua tools/call. JSON null duoc coi nhu khong truyen.
type ToolArgs struct {
	values map[string]any
}

func NewToolArgs(values map[string]any) *ToolArgs {
	if values == nil {
		values = map[string]any{}
	}
	return &ToolArgs{values: values}
}

func (a *ToolArgs) Has(name string) bool {
	value, exists := a.values[name]
	return exists && value != nil
}

// Raw tra ve gia tri tho, nil khi khong truyen (dung cho Clean()).
func (a *ToolArgs) Raw(name string) any {
	return a.values[name]
}

// Req bat buoc co va khac rong; thieu -> loi tool (khong phai loi JSON-RPC).
func (a *ToolArgs) Req(name string) string {
	value := a.Str(name, "")
	if value == "" {
		panic(argError{message: "missing required argument '" + name + "'"})
	}
	return value
}

// Str ep ve chuoi; tra ve def khi khong truyen.
func (a *ToolArgs) Str(name, def string) string {
	value := a.values[name]
	if value == nil {
		return def
	}
	return text(value)
}

// StrOrNil tra ve string neu co, nil neu khong - de dua thang vao Clean().
func (a *ToolArgs) StrOrNil(name string) any {
	value := a.values[name]
	if value == nil {
		return nil
	}
	return text(value)
}

func (a *ToolArgs) Int(name string, def int) int {
	if value, ok := a.IntOpt(name); ok {
		return value
	}
	return def
}

func (a *ToolArgs) IntOpt(name string) (int, bool) {
	value := a.values[name]
	if value == nil {
		return 0, false
	}
	converted, err := integer(value)
	if err != nil {
		panic(argError{message: "argument '" + name + "' must be an integer"})
	}
	return converted, true
}

// IntOrNil tra ve so nguyen duoi dang any, nil khi khong truyen.
func (a *ToolArgs) IntOrNil(name string) any {
	if value, ok := a.IntOpt(name); ok {
		return value
	}
	return nil
}

func (a *ToolArgs) Bool(name string, def bool) bool {
	if value, ok := a.BoolOpt(name); ok {
		return value
	}
	return def
}

func (a *ToolArgs) BoolOpt(name string) (bool, bool) {
	value := a.values[name]
	if value == nil {
		return false, false
	}
	if boolean, ok := value.(bool); ok {
		return boolean, true
	}
	switch strings.ToLower(strings.TrimSpace(text(value))) {
	case "true", "1":
		return true, true
	case "false", "0":
		return false, true
	}
	panic(argError{message: "argument '" + name + "' must be a boolean"})
}

// BoolOrNil tra ve bool duoi dang any, nil khi khong truyen.
func (a *ToolArgs) BoolOrNil(name string) any {
	if value, ok := a.BoolOpt(name); ok {
		return value
	}
	return nil
}

func (a *ToolArgs) Obj(name string) map[string]any {
	value, _ := a.values[name].(map[string]any)
	return value
}

func (a *ToolArgs) Arr(name string) []any {
	value := a.values[name]
	if value == nil {
		return nil
	}
	if array, ok := value.([]any); ok {
		return array
	}
	panic(argError{message: "argument '" + name + "' must be an array"})
}

// Matrix doc mang 2 chieu; hang khong phai mang duoc boc thanh mang mot phan tu (giong C#).
func (a *ToolArgs) Matrix(name string) [][]any {
	rows := a.Arr(name)
	if rows == nil {
		return nil
	}
	matrix := make([][]any, 0, len(rows))
	for _, row := range rows {
		if cells, ok := row.([]any); ok {
			matrix = append(matrix, cells)
			continue
		}
		matrix = append(matrix, []any{row})
	}
	return matrix
}

// Clean tao map tham so gui bridge, bo moi gia tri nil (giong _clean cua ban Python).
func Clean(pairs ...any) map[string]any {
	clean := map[string]any{}
	for index := 0; index+1 < len(pairs); index += 2 {
		if pairs[index+1] == nil {
			continue
		}
		key, _ := pairs[index].(string)
		clean[key] = pairs[index+1]
	}
	return clean
}

// text ep gia tri JSON ve chuoi theo kieu Convert.ToString(..., InvariantCulture).
func text(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'g', -1, 64)
	case nil:
		return ""
	default:
		return fmt.Sprint(typed)
	}
}

// integer ep gia tri JSON ve int theo kieu Convert.ToInt32(..., InvariantCulture).
func integer(value any) (int, error) {
	switch typed := value.(type) {
	case json.Number:
		if converted, err := typed.Int64(); err == nil {
			return int(converted), nil
		}
		asFloat, err := typed.Float64()
		if err != nil {
			return 0, err
		}
		return int(asFloat), nil
	case float64:
		return int(typed), nil
	case string:
		converted, err := strconv.Atoi(strings.TrimSpace(typed))
		if err != nil {
			return 0, err
		}
		return converted, nil
	}
	return 0, fmt.Errorf("not an integer")
}
