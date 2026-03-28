// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package drive

import (
	"strings"
	"testing"
)

func TestParseCommentDocRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantKind  string
		wantToken string
		wantErr   string
	}{
		{
			name:      "docx url",
			input:     "https://example.larksuite.com/docx/xxxxxx?from=wiki",
			wantKind:  "docx",
			wantToken: "xxxxxx",
		},
		{
			name:      "wiki url",
			input:     "https://example.larksuite.com/wiki/xxxxxx",
			wantKind:  "wiki",
			wantToken: "xxxxxx",
		},
		{
			name:      "raw token treated as docx",
			input:     "xxxxxx",
			wantKind:  "docx",
			wantToken: "xxxxxx",
		},
		{
			name:      "old doc url",
			input:     "https://example.larksuite.com/doc/xxxxxx",
			wantKind:  "doc",
			wantToken: "xxxxxx",
		},
		{
			name:    "unsupported url",
			input:   "https://example.com/not-a-doc",
			wantErr: "unsupported --doc input",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseCommentDocRef(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Kind != tt.wantKind {
				t.Fatalf("kind mismatch: want %q, got %q", tt.wantKind, got.Kind)
			}
			if got.Token != tt.wantToken {
				t.Fatalf("token mismatch: want %q, got %q", tt.wantToken, got.Token)
			}
		})
	}
}

func TestResolveCommentMode(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		explicitFull bool
		selection    string
		blockID      string
		want         commentMode
	}{
		{
			name:         "explicit full comment",
			explicitFull: true,
			want:         commentModeFull,
		},
		{
			name:         "auto full comment without anchor",
			explicitFull: false,
			want:         commentModeFull,
		},
		{
			name:      "selection means local comment",
			selection: "流程",
			want:      commentModeLocal,
		},
		{
			name:    "block id means local comment",
			blockID: "blk_123",
			want:    commentModeLocal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := resolveCommentMode(tt.explicitFull, tt.selection, tt.blockID)
			if got != tt.want {
				t.Fatalf("mode mismatch: want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestSelectLocateMatch(t *testing.T) {
	t.Parallel()

	result := locateDocResult{
		MatchCount: 2,
		Matches: []locateDocMatch{
			{
				AnchorBlockID: "blk_1",
				Blocks: []locateDocBlock{
					{BlockID: "blk_1", RawMarkdown: "流程\n"},
				},
			},
			{
				AnchorBlockID: "blk_2",
				Blocks: []locateDocBlock{
					{BlockID: "blk_2", RawMarkdown: "流程图\n"},
				},
			},
		},
	}

	_, _, err := selectLocateMatch(result)
	if err == nil || !strings.Contains(err.Error(), "matched 2 blocks") {
		t.Fatalf("expected ambiguous match error, got %v", err)
	}
	if strings.Contains(err.Error(), "流程") || strings.Contains(err.Error(), "流程图") {
		t.Fatalf("ambiguous match error should not leak locate-doc snippets: %v", err)
	}
	if !strings.Contains(err.Error(), "anchor_block_id=blk_1") || !strings.Contains(err.Error(), "anchor_block_id=blk_2") {
		t.Fatalf("ambiguous match error should keep anchor block identifiers: %v", err)
	}
}

func TestParseLocateDocResultFallsBackToFirstBlock(t *testing.T) {
	t.Parallel()

	got := parseLocateDocResult(map[string]interface{}{
		"match_count": float64(1),
		"matches": []interface{}{
			map[string]interface{}{
				"blocks": []interface{}{
					map[string]interface{}{
						"block_id":     "blk_anchor",
						"raw_markdown": "流程\n",
					},
				},
			},
		},
	})

	if len(got.Matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(got.Matches))
	}
	if got.Matches[0].AnchorBlockID != "blk_anchor" {
		t.Fatalf("expected fallback anchor block, got %q", got.Matches[0].AnchorBlockID)
	}
}

func TestParseCommentReplyElements(t *testing.T) {
	t.Parallel()

	got, err := parseCommentReplyElements(`[{"type":"text","text":"文本信息"},{"type":"mention_user","text":"ou_123"},{"type":"link","text":"https://example.com"}]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 reply elements, got %d", len(got))
	}
	if got[0]["type"] != "text" || got[0]["text"] != "文本信息" {
		t.Fatalf("unexpected text reply element: %#v", got[0])
	}
	if got[1]["type"] != "mention_user" || got[1]["mention_user"] != "ou_123" {
		t.Fatalf("unexpected mention_user reply element: %#v", got[1])
	}
	if got[2]["type"] != "link" || got[2]["link"] != "https://example.com" {
		t.Fatalf("unexpected link reply element: %#v", got[2])
	}
}

func TestParseCommentReplyElementsInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{
			name:    "invalid json",
			input:   `[{"type":"text","text":"x"}`,
			wantErr: "--content is not valid JSON",
		},
		{
			name:    "empty array",
			input:   `[]`,
			wantErr: "must contain at least one reply element",
		},
		{
			name:    "unsupported type",
			input:   `[{"type":"image","text":"x"}]`,
			wantErr: "unsupported type",
		},
		{
			name:    "mention missing value",
			input:   `[{"type":"mention_user","text":""}]`,
			wantErr: "requires text or mention_user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := parseCommentReplyElements(tt.input); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestBuildCommentCreateV2RequestFull(t *testing.T) {
	t.Parallel()

	replyElements := []map[string]interface{}{
		{
			"type": "text",
			"text": "全文评论",
		},
	}
	got := buildCommentCreateV2Request("docx", "", replyElements)

	if got["file_type"] != "docx" {
		t.Fatalf("expected file_type docx, got %#v", got["file_type"])
	}
	if _, ok := got["anchor"]; ok {
		t.Fatalf("expected no anchor for full comment, got %#v", got["anchor"])
	}

	gotReplyElements, ok := got["reply_elements"].([]map[string]interface{})
	if !ok || len(gotReplyElements) != 1 {
		t.Fatalf("expected one reply element, got %#v", got["reply_elements"])
	}
	if gotReplyElements[0]["type"] != "text" {
		t.Fatalf("expected text element, got %#v", gotReplyElements[0]["type"])
	}
	if gotReplyElements[0]["text"] != "全文评论" {
		t.Fatalf("expected text %q, got %#v", "全文评论", gotReplyElements[0]["text"])
	}
}

func TestBuildCommentCreateV2RequestLocal(t *testing.T) {
	t.Parallel()

	replyElements := []map[string]interface{}{
		{
			"type": "text",
			"text": "评论内容",
		},
	}
	got := buildCommentCreateV2Request("docx", "blk_123", replyElements)

	if got["file_type"] != "docx" {
		t.Fatalf("expected file_type docx, got %#v", got["file_type"])
	}
	anchor, ok := got["anchor"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected anchor map, got %#v", got["anchor"])
	}
	if anchor["block_id"] != "blk_123" {
		t.Fatalf("expected block_id blk_123, got %#v", anchor["block_id"])
	}

	gotReplyElements, ok := got["reply_elements"].([]map[string]interface{})
	if !ok || len(gotReplyElements) != 1 {
		t.Fatalf("expected one reply element, got %#v", got["reply_elements"])
	}
	if gotReplyElements[0]["type"] != "text" || gotReplyElements[0]["text"] != "评论内容" {
		t.Fatalf("unexpected reply element: %#v", gotReplyElements[0])
	}
}
