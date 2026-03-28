// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package output

import (
	"fmt"
	"testing"
)

func TestMarkRaw_ExitError(t *testing.T) {
	err := ErrAPI(99991672, "API error: [99991672] scope not enabled", nil)
	if err.Raw {
		t.Fatal("expected Raw=false before MarkRaw")
	}

	result := MarkRaw(err)
	if result != err {
		t.Error("expected MarkRaw to return the same error")
	}
	if !err.Raw {
		t.Error("expected Raw=true after MarkRaw")
	}
}

func TestMarkRaw_NonExitError(t *testing.T) {
	plain := fmt.Errorf("some plain error")
	result := MarkRaw(plain)
	if result != plain {
		t.Error("expected MarkRaw to return the same error for non-ExitError")
	}
}

func TestMarkRaw_Nil(t *testing.T) {
	result := MarkRaw(nil)
	if result != nil {
		t.Error("expected MarkRaw(nil) to return nil")
	}
}
