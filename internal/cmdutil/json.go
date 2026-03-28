// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmdutil

import (
	"encoding/json"

	"github.com/larksuite/cli/internal/output"
)

// ParseOptionalBody parses --data JSON for methods that accept a request body.
// Returns (nil, nil) if the method has no body or data is empty.
func ParseOptionalBody(httpMethod, data string) (interface{}, error) {
	switch httpMethod {
	case "POST", "PUT", "PATCH", "DELETE":
	default:
		return nil, nil
	}
	if data == "" {
		return nil, nil
	}
	var body interface{}
	if err := json.Unmarshal([]byte(data), &body); err != nil {
		return nil, output.ErrValidation("--data invalid JSON format")
	}
	return body, nil
}

// ParseJSONMap parses a JSON string into a map. Returns an empty map if input is empty.
func ParseJSONMap(input, label string) (map[string]any, error) {
	if input == "" {
		return map[string]any{}, nil
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(input), &result); err != nil {
		return nil, output.ErrValidation("%s invalid format, expected JSON object", label)
	}
	return result, nil
}
