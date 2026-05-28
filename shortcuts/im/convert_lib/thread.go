// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package convertlib

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/larksuite/cli/shortcuts/common"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

// ThreadRepliesPerThread is the default max replies fetched per thread in auto-expand.
const ThreadRepliesPerThread = 50

// ThreadRepliesTotalLimit is the default max total thread replies across all threads.
const ThreadRepliesTotalLimit = 500

// threadRepliesFetchConcurrency caps in-flight per-thread GET /messages calls
// when expanding multiple threads in one shortcut invocation. Each call is a
// per-thread RTT (~1s observed), so a strictly serial loop turns N=10 thread
// roots into ~10s of latency — the same multiplier that motivated the
// reactions enrichment fan-out. Capping at 4 keeps the burst well under the
// gateway-layer per-method ceiling while removing the linear stall.
const threadRepliesFetchConcurrency = 4

// ExpandThreadReplies fetches and embeds thread replies for messages that contain a thread_id.
// For each unique thread_id found in messages, it fetches up to perThread replies (asc order)
// and attaches them as "thread_replies" on the first outer message that referenced that thread.
// Expansion stops once totalLimit cumulative replies have been allocated across planned fetches.
// nameCache is the shared open_id→name map.
//
// Implementation is two-phase:
//
//  1. Plan + concurrent fetch. Walk messages in order, allocate a per-thread
//     fetch limit greedily from the remaining totalLimit budget; once budget
//     is exhausted, later threads are skipped (matching the pre-existing
//     totalLimit semantic). Then dispatch the planned fetches with bounded
//     concurrency — each fetch only writes to its own result slot, no shared
//     mutable state besides that slot.
//
//  2. Sequential attach. Walk the planned threads in their original order
//     and, for each successful result, FormatMessageItem + ResolveSenderNames
//     + AttachSenderNames the replies before attaching to the outer message.
//     This phase stays single-threaded because ResolveSenderNames writes to
//     the shared nameCache, and FormatMessageItem may trigger merge_forward
//     expansion that also touches nameCache.
//
// Compared to the pre-existing strictly-serial implementation, this trades a
// minor pessimism on totalLimit (the budget is allocated based on the
// planned per-thread limit rather than the actual returned count, so threads
// that come in under their limit don't free budget for later threads) for a
// large parallelism win on HTTP RTT — the dominant cost on busy chats with
// many thread roots. The semantic difference is documented; in the default
// configuration (totalLimit=500, perThread=50) it only matters in chats with
// >10 thread roots whose returned reply counts are well under 50, which is
// unusual.
func ExpandThreadReplies(runtime *common.RuntimeContext, messages []map[string]interface{}, nameCache map[string]string, perThread, totalLimit int) {
	if runtime == nil {
		return
	}
	if perThread < 1 {
		perThread = 1
	}
	if perThread > 50 {
		perThread = 50
	}
	if totalLimit <= 0 {
		totalLimit = ThreadRepliesTotalLimit
	}

	// Phase 1a: enumerate unique thread_ids in first-seen order and allocate
	// per-thread limits from the running budget. The first outer message
	// referencing a given thread_id is the host that will receive the
	// thread_replies attachment, matching the pre-existing behavior where
	// duplicates inherited nothing.
	type plan struct {
		threadID string
		limit    int
		host     map[string]interface{}
	}
	var plans []plan
	seen := make(map[string]bool)
	remaining := totalLimit
	for _, msg := range messages {
		tid, _ := msg["thread_id"].(string)
		if tid == "" || seen[tid] {
			continue
		}
		if remaining <= 0 {
			break
		}
		seen[tid] = true
		limit := perThread
		if limit > remaining {
			limit = remaining
		}
		plans = append(plans, plan{threadID: tid, limit: limit, host: msg})
		remaining -= limit
	}
	if len(plans) == 0 {
		return
	}

	// Phase 1b: concurrent fetch. Each goroutine writes only to its own
	// results[i] slot, so there is no shared mutable state besides that
	// slot. The single-batch fast path skips goroutine setup for clarity
	// and to keep "one thread root" behavior identical to the old code.
	type result struct {
		rawReplies []map[string]interface{}
		hasMore    bool
		err        error
	}
	results := make([]result, len(plans))
	if len(plans) == 1 {
		items, hasMore, err := fetchThreadReplies(runtime, plans[0].threadID, plans[0].limit)
		results[0] = result{rawReplies: items, hasMore: hasMore, err: err}
	} else {
		sem := make(chan struct{}, threadRepliesFetchConcurrency)
		var wg sync.WaitGroup
		for i, p := range plans {
			i, p := i, p
			sem <- struct{}{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				items, hasMore, err := fetchThreadReplies(runtime, p.threadID, p.limit)
				results[i] = result{rawReplies: items, hasMore: hasMore, err: err}
			}()
		}
		wg.Wait()
	}

	// Phase 2a: format every plan's replies sequentially. FormatMessageItem
	// may trigger a merge_forward sub-message GET (which writes to nameCache
	// via its own ResolveSenderNames call), so this stays single-threaded —
	// concurrent writes to nameCache would race.
	preparedReplies := make([][]map[string]interface{}, len(plans))
	for i, p := range plans {
		r := results[i]
		if r.err != nil || r.rawReplies == nil {
			p.host["thread_replies_error"] = true
			continue
		}
		if len(r.rawReplies) == 0 {
			continue
		}
		replies := make([]map[string]interface{}, 0, len(r.rawReplies))
		for _, raw := range r.rawReplies {
			replies = append(replies, FormatMessageItem(raw, runtime, nameCache))
		}
		preparedReplies[i] = replies
	}

	// Phase 2b: one batched ResolveSenderNames across all replies from all
	// threads. The pre-existing per-thread call pattern would issue a fresh
	// contact API request for every thread that introduced a new sender,
	// turning N threads into up to N serial contact RTTs even after the
	// fetches themselves went parallel. Consolidating into a single call
	// resolves every still-missing open_id in one request and lets the
	// nameCache absorb the rest.
	var combined []map[string]interface{}
	for _, replies := range preparedReplies {
		combined = append(combined, replies...)
	}
	if len(combined) > 0 {
		ResolveSenderNames(runtime, combined, nameCache)
	}

	// Phase 2c: attach the (now name-resolved) replies to their hosts.
	for i, p := range plans {
		replies := preparedReplies[i]
		if replies == nil {
			continue
		}
		AttachSenderNames(replies, nameCache)
		p.host["thread_replies"] = replies
		if results[i].hasMore {
			p.host["thread_has_more"] = true
		}
	}
}

// fetchThreadReplies fetches up to limit replies from a thread (ascending order).
// Returns the raw message items, whether more replies exist beyond the limit,
// and a non-nil error when the API call fails.
func fetchThreadReplies(runtime *common.RuntimeContext, threadID string, limit int) ([]map[string]interface{}, bool, error) {
	data, err := runtime.DoAPIJSON(http.MethodGet, "/open-apis/im/v1/messages", larkcore.QueryParams{
		"container_id_type":     []string{"thread"},
		"container_id":          []string{threadID},
		"sort_type":             []string{"ByCreateTimeAsc"},
		"page_size":             []string{fmt.Sprint(limit)},
		"card_msg_content_type": []string{"raw_card_content"},
	}, nil)
	if err != nil {
		return nil, false, fmt.Errorf("fetch thread replies for %s: %w", threadID, err)
	}
	hasMore, _ := data["has_more"].(bool)
	rawItems, _ := data["items"].([]interface{})
	items := make([]map[string]interface{}, 0, len(rawItems))
	for _, raw := range rawItems {
		if m, ok := raw.(map[string]interface{}); ok {
			items = append(items, m)
		}
	}
	return items, hasMore, nil
}
