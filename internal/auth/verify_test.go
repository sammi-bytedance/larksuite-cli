// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"strings"
	"testing"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"github.com/larksuite/cli/internal/httpmock"
)

func TestVerifyUserToken_TransportError(t *testing.T) {
	reg := &httpmock.Registry{}
	// Register no stubs — any request will fail with "no stub" error
	sdk := lark.NewClient("test-app", "test-secret",
		lark.WithLogLevel(larkcore.LogLevelError),
		lark.WithHttpClient(httpmock.NewClient(reg)),
	)

	err := VerifyUserToken(context.Background(), sdk, "test-token")
	if err == nil {
		t.Fatal("expected error from transport failure, got nil")
	}
}

func TestVerifyUserToken(t *testing.T) {
	tests := []struct {
		name      string
		body      interface{}
		wantErr   bool
		errSubstr string
	}{
		{
			name:    "success",
			body:    map[string]interface{}{"code": 0, "msg": "ok"},
			wantErr: false,
		},
		{
			name:      "token invalid",
			body:      map[string]interface{}{"code": 99991668, "msg": "invalid token"},
			wantErr:   true,
			errSubstr: "[99991668]",
		},
		{
			name:      "non-JSON response",
			body:      "not json",
			wantErr:   true,
			errSubstr: "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &httpmock.Registry{}
			t.Cleanup(func() { reg.Verify(t) })

			reg.Register(&httpmock.Stub{
				Method: "GET",
				URL:    "/open-apis/authen/v1/user_info",
				Body:   tt.body,
			})

			sdk := lark.NewClient("test-app", "test-secret",
				lark.WithLogLevel(larkcore.LogLevelError),
				lark.WithHttpClient(httpmock.NewClient(reg)),
			)

			err := VerifyUserToken(context.Background(), sdk, "test-token")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.errSubstr) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstr)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
