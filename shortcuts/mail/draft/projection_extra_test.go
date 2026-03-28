// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package draft

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Projection with attachment summaries
// ---------------------------------------------------------------------------

func TestProjectAttachmentSummary(t *testing.T) {
	snapshot := mustParseFixtureDraft(t, mustReadFixture(t, "testdata/forward_draft.eml"))
	proj := Project(snapshot)

	if len(proj.AttachmentsSummary) == 0 {
		t.Fatalf("AttachmentsSummary should not be empty for forward_draft")
	}
	found := false
	for _, att := range proj.AttachmentsSummary {
		if att.FileName != "" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected at least one attachment with filename")
	}
}

// ---------------------------------------------------------------------------
// Projection with plain-text only draft
// ---------------------------------------------------------------------------

func TestProjectPlainTextDraft(t *testing.T) {
	snapshot := mustParseFixtureDraft(t, `Subject: Plain
From: Alice <alice@example.com>
To: Bob <bob@example.com>
MIME-Version: 1.0
Content-Type: text/plain; charset=UTF-8

This is a plain text body.
`)
	proj := Project(snapshot)
	if proj.Subject != "Plain" {
		t.Fatalf("Subject = %q", proj.Subject)
	}
	if proj.BodyText != "This is a plain text body.\n" {
		t.Fatalf("BodyText = %q", proj.BodyText)
	}
	if proj.BodyHTMLSummary != "" {
		t.Fatalf("BodyHTMLSummary should be empty for plain-text draft")
	}
}

// ---------------------------------------------------------------------------
// Projection with encoding problem
// ---------------------------------------------------------------------------

func TestProjectEncodingProblemWarning(t *testing.T) {
	snapshot := mustParseFixtureDraft(t, `Subject: Bad
From: Alice <alice@example.com>
To: Bob <bob@example.com>
MIME-Version: 1.0
Content-Type: multipart/mixed; boundary=mix

--mix
Content-Type: text/plain; charset=UTF-8

hello
--mix
Content-Type: application/pdf; name=report.pdf
Content-Disposition: attachment; filename=report.pdf
Content-Transfer-Encoding: base64

!!!not-valid-base64!!!
--mix--
`)
	proj := Project(snapshot)
	foundWarning := false
	for _, w := range proj.Warnings {
		if len(w) > 0 {
			foundWarning = true
		}
	}
	if !foundWarning {
		t.Fatalf("expected encoding problem warning, Warnings = %v", proj.Warnings)
	}
}

// ---------------------------------------------------------------------------
// Projection truncates long HTML
// ---------------------------------------------------------------------------

func TestProjectHTMLSummaryTruncation(t *testing.T) {
	longHTML := "<p>" + string(make([]byte, 500)) + "</p>"
	snapshot := mustParseFixtureDraft(t, `Subject: Long
From: Alice <alice@example.com>
To: Bob <bob@example.com>
MIME-Version: 1.0
Content-Type: text/html; charset=UTF-8

`+longHTML+`
`)
	proj := Project(snapshot)
	if len(proj.BodyHTMLSummary) > 300 {
		t.Fatalf("BodyHTMLSummary len = %d, should be truncated", len(proj.BodyHTMLSummary))
	}
}
