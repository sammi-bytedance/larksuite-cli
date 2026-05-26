// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package convertlib

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

// TestEnrichReactions_Success exercises the basic happy path: messages that
// carry reactions get a "reactions" field, messages without reactions stay
// untouched.
func TestEnrichReactions_Success(t *testing.T) {
	runtime := newBotConvertlibRuntime(t, convertlibRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if !strings.Contains(req.URL.Path, "/open-apis/im/v1/messages/reactions/batch_query") {
			return nil, fmt.Errorf("unexpected path: %s", req.URL.Path)
		}
		var payload map[string]interface{}
		body, _ := io.ReadAll(req.Body)
		_ = json.Unmarshal(body, &payload)
		queries, _ := payload["queries"].([]interface{})
		if len(queries) != 2 {
			t.Fatalf("queries size = %d, want 2", len(queries))
		}
		return convertlibJSONResponse(200, map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"success_msg_reaction_counts": []interface{}{
					map[string]interface{}{
						"message_id": "om_a",
						"reaction_count": []interface{}{
							map[string]interface{}{"reaction_type": "SMILE", "count": 3},
						},
					},
				},
				"success_msg_reaction_details": []interface{}{
					map[string]interface{}{
						"message_id": "om_a",
						"message_reaction_items": []interface{}{
							map[string]interface{}{
								"reaction_id": "react_1",
								"emoji_type":  "SMILE",
								"operator":    map[string]interface{}{"operator_id": "ou_x", "operator_type": "user"},
								"action_time": "1710600000",
							},
						},
					},
				},
				"fail_msg_reaction_details": []interface{}{},
			},
		}), nil
	}))

	messages := []map[string]interface{}{
		{"message_id": "om_a"},
		{"message_id": "om_b"},
	}

	EnrichReactions(runtime, messages)

	reactionsA, ok := messages[0]["reactions"].(map[string]interface{})
	if !ok {
		t.Fatalf("message om_a missing reactions field: %#v", messages[0])
	}
	counts, _ := reactionsA["counts"].([]interface{})
	if len(counts) != 1 {
		t.Fatalf("om_a counts = %d, want 1", len(counts))
	}
	details, _ := reactionsA["details"].([]interface{})
	if len(details) != 1 {
		t.Fatalf("om_a details = %d, want 1", len(details))
	}

	if _, ok := messages[1]["reactions"]; ok {
		t.Fatalf("message om_b should not have reactions field (none in response): %#v", messages[1])
	}
}

// TestEnrichReactions_BatchSize splits queries into batches of 20 (server-side
// max for batch_query).
func TestEnrichReactions_BatchSize(t *testing.T) {
	var observedBatchSizes []int
	runtime := newBotConvertlibRuntime(t, convertlibRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		var payload map[string]interface{}
		_ = json.Unmarshal(body, &payload)
		queries, _ := payload["queries"].([]interface{})
		observedBatchSizes = append(observedBatchSizes, len(queries))
		return convertlibJSONResponse(200, map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{},
		}), nil
	}))

	messages := make([]map[string]interface{}, 25)
	for i := range messages {
		messages[i] = map[string]interface{}{"message_id": fmt.Sprintf("om_%02d", i)}
	}

	EnrichReactions(runtime, messages)

	if want := []int{20, 5}; !reflect.DeepEqual(observedBatchSizes, want) {
		t.Fatalf("batch sizes = %v, want %v", observedBatchSizes, want)
	}
}

// TestEnrichReactions_APIFailure: when the API call fails, messages stay as-is
// (no reactions field) and no panic occurs.
func TestEnrichReactions_APIFailure(t *testing.T) {
	runtime := newBotConvertlibRuntime(t, convertlibRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("simulated network error")
	}))

	messages := []map[string]interface{}{
		{"message_id": "om_a"},
		{"message_id": "om_b"},
	}

	EnrichReactions(runtime, messages)

	for _, m := range messages {
		if _, ok := m["reactions"]; ok {
			t.Fatalf("message %v should have no reactions after API failure", m["message_id"])
		}
	}
}

// TestEnrichReactions_EmptyMessages: no messages -> no API call at all.
func TestEnrichReactions_EmptyMessages(t *testing.T) {
	called := false
	runtime := newBotConvertlibRuntime(t, convertlibRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		called = true
		return convertlibJSONResponse(200, map[string]interface{}{"code": 0, "data": map[string]interface{}{}}), nil
	}))

	EnrichReactions(runtime, nil)
	EnrichReactions(runtime, []map[string]interface{}{})

	if called {
		t.Fatalf("API should not be called when messages list is empty")
	}
}

// TestEnrichReactions_SkipsMessagesWithoutID: messages missing message_id
// (defensive) should not crash and not be sent in queries.
func TestEnrichReactions_SkipsMessagesWithoutID(t *testing.T) {
	var sentIDs []string
	runtime := newBotConvertlibRuntime(t, convertlibRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		body, _ := io.ReadAll(req.Body)
		var payload map[string]interface{}
		_ = json.Unmarshal(body, &payload)
		queries, _ := payload["queries"].([]interface{})
		for _, q := range queries {
			qm, _ := q.(map[string]interface{})
			id, _ := qm["message_id"].(string)
			sentIDs = append(sentIDs, id)
		}
		return convertlibJSONResponse(200, map[string]interface{}{"code": 0, "data": map[string]interface{}{}}), nil
	}))

	messages := []map[string]interface{}{
		{"message_id": "om_a"},
		{}, // no message_id
		{"message_id": ""},
		{"message_id": "om_b"},
	}

	EnrichReactions(runtime, messages)

	if want := []string{"om_a", "om_b"}; !reflect.DeepEqual(sentIDs, want) {
		t.Fatalf("sent IDs = %v, want %v", sentIDs, want)
	}
}
