// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package event

import (
	"context"
	"encoding/json"
)

// CardActionTriggerProcessor handles card.action.trigger events.
//
// Compact output fields:
//   - type, event_id, timestamp
//   - operator_id: open_id of the user who triggered the action
//   - action_tag: component tag (e.g. "button", "select_static", "input")
//   - action_value: custom value map attached to the component
//   - chat_id, message_id: context of the card (if available)
type CardActionTriggerProcessor struct{}

func (p *CardActionTriggerProcessor) EventType() string { return "card.action.trigger" }

func (p *CardActionTriggerProcessor) Transform(_ context.Context, raw *RawEvent, mode TransformMode) interface{} {
	if mode == TransformRaw {
		return raw
	}

	var ev struct {
		Operator struct {
			OpenID string `json:"open_id"`
			UserID string `json:"user_id"`
		} `json:"operator"`
		Action struct {
			Tag        string                 `json:"tag"`
			Value      map[string]interface{} `json:"value"`
			Option     string                 `json:"option"`
			InputValue string                 `json:"input_value"`
			Checked    *bool                  `json:"checked"`
		} `json:"action"`
		Context struct {
			OpenMessageID string `json:"open_message_id"`
			OpenChatID    string `json:"open_chat_id"`
		} `json:"context"`
	}
	if err := json.Unmarshal(raw.Event, &ev); err != nil {
		return raw
	}

	out := compactBase(raw)
	if ev.Operator.OpenID != "" {
		out["operator_id"] = ev.Operator.OpenID
	}
	if ev.Action.Tag != "" {
		out["action_tag"] = ev.Action.Tag
	}
	if len(ev.Action.Value) > 0 {
		out["action_value"] = ev.Action.Value
	}
	if ev.Action.Option != "" {
		out["action_option"] = ev.Action.Option
	}
	if ev.Action.InputValue != "" {
		out["input_value"] = ev.Action.InputValue
	}
	if ev.Action.Checked != nil {
		out["checked"] = *ev.Action.Checked
	}
	if ev.Context.OpenChatID != "" {
		out["chat_id"] = ev.Context.OpenChatID
	}
	if ev.Context.OpenMessageID != "" {
		out["message_id"] = ev.Context.OpenMessageID
	}
	return out
}

func (p *CardActionTriggerProcessor) DeduplicateKey(raw *RawEvent) string { return raw.Header.EventID }
func (p *CardActionTriggerProcessor) WindowStrategy() WindowConfig        { return WindowConfig{} }
