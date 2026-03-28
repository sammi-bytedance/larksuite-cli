// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package common

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/larksuite/cli/internal/output"
)

func TestEnhancePermissionError_MissingScopeType(t *testing.T) {
	scopes := []string{"calendar:calendar:read"}
	err := &output.ExitError{
		Code:   1,
		Detail: &output.ErrDetail{Type: "missing_scope", Message: "missing scope"},
	}
	got := enhancePermissionError(err, scopes)
	var exitErr *output.ExitError
	if !errors.As(got, &exitErr) {
		t.Fatalf("expected ExitError, got %T", got)
	}
	if exitErr.Detail.Hint == "" {
		t.Error("expected hint for missing_scope type")
	}
	if !strings.Contains(exitErr.Detail.Hint, "calendar:calendar:read") {
		t.Errorf("hint %q missing scope info", exitErr.Detail.Hint)
	}
}

func TestEnhancePermissionError_KeywordPermission(t *testing.T) {
	scopes := []string{"drive:drive:read"}
	err := &output.ExitError{
		Code:   1,
		Detail: &output.ErrDetail{Type: "api_error", Message: "Permission denied for resource"},
	}
	got := enhancePermissionError(err, scopes)
	var exitErr *output.ExitError
	if !errors.As(got, &exitErr) {
		t.Fatalf("expected ExitError, got %T", got)
	}
	if !strings.Contains(exitErr.Detail.Hint, "drive:drive:read") {
		t.Errorf("hint %q missing scope info", exitErr.Detail.Hint)
	}
}

func TestEnhancePermissionError_KeywordScope(t *testing.T) {
	scopes := []string{"task:task:read"}
	err := &output.ExitError{
		Code:   1,
		Detail: &output.ErrDetail{Type: "api_error", Message: "Insufficient scope for operation"},
	}
	got := enhancePermissionError(err, scopes)
	var exitErr *output.ExitError
	if !errors.As(got, &exitErr) {
		t.Fatalf("expected ExitError, got %T", got)
	}
	if !strings.Contains(exitErr.Detail.Hint, "task:task:read") {
		t.Errorf("hint %q missing scope info", exitErr.Detail.Hint)
	}
}

func TestEnhancePermissionError_KeywordAuthorization(t *testing.T) {
	scopes := []string{"contact:contact:read"}
	err := &output.ExitError{
		Code:   1,
		Detail: &output.ErrDetail{Type: "api_error", Message: "Authorization required"},
	}
	got := enhancePermissionError(err, scopes)
	var exitErr *output.ExitError
	if !errors.As(got, &exitErr) {
		t.Fatalf("expected ExitError, got %T", got)
	}
	if !strings.Contains(exitErr.Detail.Hint, "contact:contact:read") {
		t.Errorf("hint %q missing scope info", exitErr.Detail.Hint)
	}
}

func TestEnhancePermissionError(t *testing.T) {
	scopes := []string{"calendar:calendar:read", "drive:drive:read"}

	tests := []struct {
		name       string
		err        error
		wantHint   bool
		hintSubstr string
	}{
		{
			name: "permission type gets enhanced",
			err: &output.ExitError{
				Code:   1,
				Detail: &output.ErrDetail{Type: "permission", Message: "no permission"},
			},
			wantHint:   true,
			hintSubstr: "scope",
		},
		{
			name: "mcp_error with unauthorized keyword gets enhanced",
			err: &output.ExitError{
				Code:   1,
				Detail: &output.ErrDetail{Type: "mcp_error", Message: "request unauthorized by server"},
			},
			wantHint:   true,
			hintSubstr: "scope",
		},
		{
			name: "api_error without keyword not modified",
			err: &output.ExitError{
				Code:   1,
				Detail: &output.ErrDetail{Type: "api_error", Message: "timeout"},
			},
			wantHint: false,
		},
		{
			name:     "plain error not modified",
			err:      fmt.Errorf("plain error"),
			wantHint: false,
		},
		{
			name: "nil Detail not modified",
			err: &output.ExitError{
				Code:   1,
				Detail: nil,
			},
			wantHint: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := enhancePermissionError(tt.err, scopes)

			if !tt.wantHint {
				// Should return original error unchanged
				if got != tt.err {
					t.Errorf("expected original error returned, got different error: %v", got)
				}
				return
			}

			// Should return an enhanced ExitError with a hint
			var exitErr *output.ExitError
			if !errors.As(got, &exitErr) {
				t.Fatalf("expected ExitError, got %T: %v", got, got)
			}
			if exitErr.Detail == nil {
				t.Fatal("expected Detail to be non-nil")
			}
			if exitErr.Detail.Hint == "" {
				t.Fatal("expected non-empty hint")
			}
			if !strings.Contains(exitErr.Detail.Hint, tt.hintSubstr) {
				t.Errorf("hint %q does not contain %q", exitErr.Detail.Hint, tt.hintSubstr)
			}
			// Verify the hint includes the actual scopes
			for _, s := range scopes {
				if !strings.Contains(exitErr.Detail.Hint, s) {
					t.Errorf("hint %q does not contain scope %q", exitErr.Detail.Hint, s)
				}
			}
		})
	}
}
