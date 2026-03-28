// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package calendar

import (
	"time"

	"github.com/larksuite/cli/shortcuts/common"
)

const (
	PrimaryCalendarIDStr = "primary"
)

// resolveStartEnd returns (startInput, endInput) from flags with defaults.
// --start defaults to today's date, --end defaults to start date (will be resolved to end-of-day by caller).
func resolveStartEnd(runtime *common.RuntimeContext) (string, string) {
	startInput := runtime.Str("start")
	if startInput == "" {
		startInput = time.Now().Format("2006-01-02")
	}
	endInput := runtime.Str("end")
	if endInput == "" {
		endInput = startInput
	}
	return startInput, endInput
}
