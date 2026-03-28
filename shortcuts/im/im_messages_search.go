// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package im

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/larksuite/cli/internal/output"
	"github.com/larksuite/cli/shortcuts/common"
	convertlib "github.com/larksuite/cli/shortcuts/im/convert_lib"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
)

var ImMessagesSearch = common.Shortcut{
	Service:     "im",
	Command:     "+messages-search",
	Description: "Search messages across chats (supports keyword, sender, time range filters) with user identity; user-only; filters by chat/sender/attachment/time, enriches results via mget and chats batch_query",
	Risk:        "read",
	Scopes:      []string{"search:message", "contact:user.basic_profile:readonly"},
	AuthTypes:   []string{"user"},
	HasFormat:   true,
	Flags: []common.Flag{
		{Name: "query", Desc: "search keyword"},
		{Name: "chat-id", Desc: "limit to chat IDs, comma-separated"},
		{Name: "sender", Desc: "sender open_ids, comma-separated"},
		{Name: "include-attachment-type", Desc: "include attachment type filter", Enum: []string{"file", "image", "video", "link"}},
		{Name: "chat-type", Desc: "chat type", Enum: []string{"group", "p2p"}},
		{Name: "sender-type", Desc: "sender type", Enum: []string{"user", "bot"}},
		{Name: "exclude-sender-type", Desc: "exclude sender type", Enum: []string{"user", "bot"}},
		{Name: "is-at-me", Type: "bool", Desc: "only messages that @me"},
		{Name: "start", Desc: "start time(ISO 8601) with local timezone offset (e.g. 2026-03-24T00:00:00+08:00)"},
		{Name: "end", Desc: "end time(ISO 8601) with local timezone offset (e.g. 2026-03-25T23:59:59+08:00)"},
		{Name: "page-size", Default: "20", Desc: "page size (1-50)"},
		{Name: "page-token", Desc: "page token"},
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		req, err := buildMessagesSearchRequest(runtime)
		if err != nil {
			return common.NewDryRunAPI().Desc(err.Error())
		}
		dryParams := make(map[string]interface{}, len(req.params))
		for k, vs := range req.params {
			if len(vs) > 0 {
				dryParams[k] = vs[0]
			}
		}
		return common.NewDryRunAPI().
			Desc("Step 1: search messages").
			POST("/open-apis/im/v1/messages/search").
			Params(dryParams).
			Body(req.body).
			Desc("Step 2 (if results): GET /open-apis/im/v1/messages/mget?message_ids=...  — batch fetch message details (max 50)").
			Desc("Step 3 (if results): POST /open-apis/im/v1/chats/batch_query  — fetch chat names for context")
	},
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		_, err := buildMessagesSearchRequest(runtime)
		return err
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		req, err := buildMessagesSearchRequest(runtime)
		if err != nil {
			return err
		}

		searchData, err := runtime.DoAPIJSON(http.MethodPost, "/open-apis/im/v1/messages/search", req.params, req.body)
		if err != nil {
			return err
		}
		rawItems, _ := searchData["items"].([]interface{})
		hasMore, nextPageToken := common.PaginationMeta(searchData)

		if len(rawItems) == 0 {
			outData := map[string]interface{}{
				"messages":   []interface{}{},
				"total":      0,
				"has_more":   hasMore,
				"page_token": nextPageToken,
			}
			runtime.OutFormat(outData, nil, func(w io.Writer) {
				fmt.Fprintln(w, "No matching messages found.")
			})
			return nil
		}

		messageIds := make([]string, 0, len(rawItems))
		for _, item := range rawItems {
			if itemMap, ok := item.(map[string]interface{}); ok {
				if metaData, ok := itemMap["meta_data"].(map[string]interface{}); ok {
					if id, ok := metaData["message_id"].(string); ok && id != "" {
						messageIds = append(messageIds, id)
					}
				}
			}
		}

		// ── Step 2: Batch fetch message details (mget) ──
		mgetURL := buildMGetURL(messageIds)
		mgetData, err := runtime.DoAPIJSON(http.MethodGet, mgetURL, nil, nil)
		if err != nil {
			// Fallback when mget fails: return ID list only
			outData := map[string]interface{}{
				"message_ids": messageIds,
				"total":       len(messageIds),
				"has_more":    hasMore,
				"page_token":  nextPageToken,
				"note":        "failed to fetch message details, returning ID list only",
			}
			runtime.OutFormat(outData, nil, func(w io.Writer) {
				fmt.Fprintf(w, "Found %d messages (failed to fetch details):\n", len(messageIds))
				for _, id := range messageIds {
					fmt.Fprintln(w, " ", id)
				}
			})
			return nil
		}
		msgItems, _ := mgetData["items"].([]interface{})

		// ── Step 3: Batch fetch chat info ──
		chatIdSet := map[string]bool{}
		for _, item := range msgItems {
			m, _ := item.(map[string]interface{})
			if chatId, _ := m["chat_id"].(string); chatId != "" {
				chatIdSet[chatId] = true
			}
		}
		chatContexts := map[string]map[string]interface{}{}
		if len(chatIdSet) > 0 {
			chatIds := make([]string, 0, len(chatIdSet))
			for id := range chatIdSet {
				chatIds = append(chatIds, id)
			}
			chatRes, chatErr := runtime.DoAPIJSON(
				http.MethodPost, "/open-apis/im/v1/chats/batch_query",
				larkcore.QueryParams{"user_id_type": []string{"open_id"}},
				map[string]interface{}{"chat_ids": chatIds},
			)
			if chatErr == nil {
				if chatItems, ok := chatRes["items"].([]interface{}); ok {
					for _, ci := range chatItems {
						cm, _ := ci.(map[string]interface{})
						if cid, _ := cm["chat_id"].(string); cid != "" {
							chatContexts[cid] = cm
						}
					}
				}
			}
		}

		// ── Step 4: Format message content + attach chat context ──
		nameCache := make(map[string]string)
		enriched := make([]map[string]interface{}, 0, len(msgItems))
		for _, item := range msgItems {
			m, _ := item.(map[string]interface{})
			chatId, _ := m["chat_id"].(string)

			// Reuse unified content converter
			msg := convertlib.FormatMessageItem(m, runtime, nameCache)
			if chatId != "" {
				msg["chat_id"] = chatId
			}
			if chatCtx, ok := chatContexts[chatId]; ok {
				chatMode, _ := chatCtx["chat_mode"].(string)
				chatName, _ := chatCtx["name"].(string)
				if chatMode == "p2p" {
					msg["chat_type"] = "p2p"
					if p2pId, _ := chatCtx["p2p_target_id"].(string); p2pId != "" {
						msg["chat_partner"] = map[string]interface{}{"open_id": p2pId}
					}
				} else {
					msg["chat_type"] = chatMode
					if chatName != "" {
						msg["chat_name"] = chatName
					}
				}
			}
			enriched = append(enriched, msg)
		}

		// Enrich: resolve sender names for outer messages (reuses cache from merge_forward)
		convertlib.ResolveSenderNames(runtime, enriched, nameCache)
		convertlib.AttachSenderNames(enriched, nameCache)

		outData := map[string]interface{}{
			"messages":   enriched,
			"total":      len(enriched),
			"has_more":   hasMore,
			"page_token": nextPageToken,
		}
		runtime.OutFormat(outData, nil, func(w io.Writer) {
			if len(enriched) == 0 {
				fmt.Fprintln(w, "No matching messages found.")
				return
			}
			var rows []map[string]interface{}
			for _, msg := range enriched {
				row := map[string]interface{}{
					"time": msg["create_time"],
					"type": msg["msg_type"],
				}
				if sender, ok := msg["sender"].(map[string]interface{}); ok {
					if name, _ := sender["name"].(string); name != "" {
						row["sender"] = name
					}
				}
				if chatName, ok := msg["chat_name"].(string); ok && chatName != "" {
					row["chat"] = chatName
				} else if chatType, ok := msg["chat_type"].(string); ok && chatType == "p2p" {
					row["chat"] = "p2p"
				} else if cid, ok := msg["chat_id"].(string); ok {
					row["chat"] = cid
				}
				if content, _ := msg["content"].(string); content != "" {
					row["content"] = convertlib.TruncateContent(content, 30)
				}
				rows = append(rows, row)
			}
			output.PrintTable(w, rows)
			moreHint := ""
			if hasMore {
				moreHint = " (more available, use --page-token to fetch next page)"
			}
			fmt.Fprintf(w, "\n%d search result(s)%s\n", len(enriched), moreHint)
		})
		return nil
	},
}

type messagesSearchRequest struct {
	params larkcore.QueryParams
	body   map[string]interface{}
}

func buildMessagesSearchRequest(runtime *common.RuntimeContext) (*messagesSearchRequest, error) {
	query := runtime.Str("query")
	chatFlag := runtime.Str("chat-id")
	senderFlag := runtime.Str("sender")
	includeAttachmentTypeFlag := runtime.Str("include-attachment-type")
	chatTypeFlag := runtime.Str("chat-type")
	senderTypeFlag := runtime.Str("sender-type")
	excludeSenderTypeFlag := runtime.Str("exclude-sender-type")
	startFlag := runtime.Str("start")
	endFlag := runtime.Str("end")
	pageSizeStr := runtime.Str("page-size")
	pageToken := runtime.Str("page-token")

	filter := map[string]interface{}{}
	timeRange := map[string]interface{}{}
	var startTs, endTs string
	if startFlag != "" {
		ts, err := common.ParseTime(startFlag)
		if err != nil {
			return nil, output.ErrValidation("--start: %v", err)
		}
		startTs = ts
		start := startFlag
		timeRange["start_time"] = start
	}
	if endFlag != "" {
		ts, err := common.ParseTime(endFlag, "end")
		if err != nil {
			return nil, output.ErrValidation("--end: %v", err)
		}
		endTs = ts
		end := endFlag
		timeRange["end_time"] = end
	}
	if startTs != "" && endTs != "" {
		sv, _ := strconv.ParseInt(startTs, 10, 64)
		ev, _ := strconv.ParseInt(endTs, 10, 64)
		if sv > ev {
			return nil, output.ErrValidation("--start cannot be later than --end")
		}
	}
	if len(timeRange) > 0 {
		filter["time_range"] = timeRange
	}

	if senderTypeFlag != "" && excludeSenderTypeFlag != "" {
		if senderTypeFlag == excludeSenderTypeFlag {
			return nil, output.ErrValidation("--sender-type and --exclude-sender-type cannot be the same value")
		}
	}
	if chatFlag != "" {
		for _, chatID := range common.SplitCSV(chatFlag) {
			if _, err := common.ValidateChatID(chatID); err != nil {
				return nil, err
			}
		}
		filter["chat_ids"] = common.SplitCSV(chatFlag)
	}
	if senderFlag != "" {
		for _, userID := range common.SplitCSV(senderFlag) {
			if _, err := common.ValidateUserID(userID); err != nil {
				return nil, err
			}
		}
		filter["from_ids"] = common.SplitCSV(senderFlag)
	}
	if includeAttachmentTypeFlag != "" {
		filter["include_attachment_types"] = []string{includeAttachmentTypeFlag}
	}
	if senderTypeFlag != "" {
		filter["from_types"] = []string{senderTypeFlag}
	}
	if excludeSenderTypeFlag != "" {
		filter["exclude_from_types"] = []string{excludeSenderTypeFlag}
	}
	if chatTypeFlag != "" {
		filter["chat_type"] = chatTypeFlag
	}
	if runtime.Bool("is-at-me") {
		filter["is_at_me"] = true
	}

	body := map[string]interface{}{"query": query}
	if len(filter) > 0 {
		body["filter"] = filter
	}

	pageSize := 20
	if pageSizeStr != "" {
		n, err := strconv.Atoi(pageSizeStr)
		if err != nil || n < 1 {
			return nil, output.ErrValidation("--page-size must be an integer between 1 and 50")
		}
		if n > 50 {
			n = 50
		}
		pageSize = n
	}

	params := larkcore.QueryParams{
		"page_size": []string{strconv.Itoa(pageSize)},
	}
	if pageToken != "" {
		params["page_token"] = []string{pageToken}
	}

	return &messagesSearchRequest{
		params: params,
		body:   body,
	}, nil
}
