// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package convertlib

import (
	"fmt"
	"net/http"

	"github.com/larksuite/cli/shortcuts/common"
)

// reactionsBatchQueryMaxQueries is the server-side hard limit on queries[]
// length for POST /im/v1/messages/reactions/batch_query (see
// larkim/message/members/facade_reaction/service: batchListReactionsMaxMessageIDs).
const reactionsBatchQueryMaxQueries = 20

// EnrichReactions enriches messages with their reactions by calling the
// im.reactions.batch_query API. Messages are modified in place: each message
// that the server returns reactions for gets a "reactions" map attached.
//
// Failure modes (warning to stderr + skip; never aborts main message output):
//   - batch_query call fails (network, 5xx, scope insufficient, rate limited):
//     messages in the failed batch stay without a "reactions" field.
//   - batch_query returns a partial result: only messages that succeeded get
//     the field; failed ones stay without it.
//
// Output shape (only on messages that the server actually returned data for):
//
//	"reactions": {
//	  "counts":  [{"reaction_type": "SMILE", "count": 3}],
//	  "details": [{"reaction_id": "...", "emoji_type": "SMILE",
//	                "operator": {...}, "action_time": "..."}]
//	}
//
// The server caps queries[] at 20 per call, so messages are split into
// batches of size <= 20 before invoking the API.
func EnrichReactions(runtime *common.RuntimeContext, messages []map[string]interface{}) {
	if len(messages) == 0 {
		return
	}

	// Index messages by ID so we can merge reactions back later.
	idIndex := make(map[string]map[string]interface{}, len(messages))
	var ids []string
	for _, msg := range messages {
		id, _ := msg["message_id"].(string)
		if id == "" {
			continue
		}
		if _, dup := idIndex[id]; dup {
			continue
		}
		idIndex[id] = msg
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return
	}

	for i := 0; i < len(ids); i += reactionsBatchQueryMaxQueries {
		end := i + reactionsBatchQueryMaxQueries
		if end > len(ids) {
			end = len(ids)
		}
		fetchReactionsBatch(runtime, ids[i:end], idIndex)
	}
}

// fetchReactionsBatch invokes batch_query for one batch of <= 20 message IDs
// and merges the results into idIndex. Failures are logged to stderr without
// aborting subsequent batches.
func fetchReactionsBatch(runtime *common.RuntimeContext, batchIDs []string, idIndex map[string]map[string]interface{}) {
	queries := make([]map[string]interface{}, 0, len(batchIDs))
	for _, id := range batchIDs {
		queries = append(queries, map[string]interface{}{"message_id": id})
	}

	data, err := runtime.DoAPIJSON(http.MethodPost,
		"/open-apis/im/v1/messages/reactions/batch_query",
		nil,
		map[string]interface{}{"queries": queries},
	)
	if err != nil {
		fmt.Fprintf(runtime.IO().ErrOut, "warning: reactions_batch_query_failed: %v\n", err)
		return
	}

	countsByMsg := groupReactionCounts(data["success_msg_reaction_counts"])
	detailsByMsg := groupReactionDetails(data["success_msg_reaction_details"])

	// Attach the merged reactions block to each message that had any data.
	for _, id := range batchIDs {
		msg, ok := idIndex[id]
		if !ok {
			continue
		}
		counts := countsByMsg[id]
		details := detailsByMsg[id]
		if len(counts) == 0 && len(details) == 0 {
			continue
		}
		block := make(map[string]interface{}, 2)
		if len(counts) > 0 {
			block["counts"] = counts
		}
		if len(details) > 0 {
			block["details"] = details
		}
		msg["reactions"] = block
	}

	// Surface per-message failures from the API response.
	if fails, _ := data["fail_msg_reaction_details"].([]interface{}); len(fails) > 0 {
		var failedIDs []string
		for _, raw := range fails {
			item, _ := raw.(map[string]interface{})
			if id, _ := item["message_id"].(string); id != "" {
				failedIDs = append(failedIDs, id)
			}
		}
		if len(failedIDs) > 0 {
			fmt.Fprintf(runtime.IO().ErrOut,
				"warning: reactions_partial_failed: %d message(s) failed (%v)\n",
				len(failedIDs), failedIDs)
		}
	}
}

func groupReactionCounts(raw interface{}) map[string][]interface{} {
	groups := map[string][]interface{}{}
	items, _ := raw.([]interface{})
	for _, item := range items {
		row, _ := item.(map[string]interface{})
		msgID, _ := row["message_id"].(string)
		if msgID == "" {
			continue
		}
		entries, _ := row["reaction_count"].([]interface{})
		if len(entries) == 0 {
			continue
		}
		groups[msgID] = append(groups[msgID], entries...)
	}
	return groups
}

func groupReactionDetails(raw interface{}) map[string][]interface{} {
	groups := map[string][]interface{}{}
	items, _ := raw.([]interface{})
	for _, item := range items {
		row, _ := item.(map[string]interface{})
		msgID, _ := row["message_id"].(string)
		if msgID == "" {
			continue
		}
		entries, _ := row["message_reaction_items"].([]interface{})
		if len(entries) == 0 {
			continue
		}
		groups[msgID] = append(groups[msgID], entries...)
	}
	return groups
}
