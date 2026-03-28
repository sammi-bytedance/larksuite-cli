// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package common

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestParseTimeISO(t *testing.T) {
	s, err := ParseTime("2026-01-15")
	if err != nil {
		t.Fatalf("ParseTime(date) error: %v", err)
	}
	ts, _ := strconv.ParseInt(s, 10, 64)
	parsed := time.Unix(ts, 0)
	if parsed.Year() != 2026 || parsed.Month() != 1 || parsed.Day() != 15 {
		t.Errorf("ParseTime(2026-01-15) = %v", parsed)
	}
}

func TestParseTimeUnix(t *testing.T) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	s, err := ParseTime(ts)
	if err != nil {
		t.Fatalf("ParseTime(unix) error: %v", err)
	}
	if s != ts {
		t.Errorf("ParseTime(%q) = %q, want pass-through", ts, s)
	}
}

func TestParseTimeRejectsRelative(t *testing.T) {
	for _, input := range []string{"today", "tomorrow", "yesterday", "now", "this_week", "+3d", "-1w", "+2h", "-30m", "last_7_days"} {
		t.Run(input, func(t *testing.T) {
			_, err := ParseTime(input)
			if err == nil {
				t.Errorf("ParseTime(%q) should return error, but got nil", input)
			}
		})
	}
}

func TestParseTimeEndHint(t *testing.T) {
	s, err := ParseTime("2026-03-15", "end")
	if err != nil {
		t.Fatalf("ParseTime(date, end) error: %v", err)
	}
	ts, _ := strconv.ParseInt(s, 10, 64)
	parsed := time.Unix(ts, 0)
	if parsed.Hour() != 23 || parsed.Minute() != 59 || parsed.Second() != 59 {
		t.Errorf("ParseTime(2026-03-15, end) = %v, want 23:59:59", parsed)
	}
}

func TestEnsureWritableFile(t *testing.T) {
	t.Run("allows missing target", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing.txt")
		if err := EnsureWritableFile(path, false); err != nil {
			t.Fatalf("EnsureWritableFile() unexpected error: %v", err)
		}
	})

	t.Run("rejects existing target without overwrite", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "exists.txt")
		if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}
		if err := EnsureWritableFile(path, false); err == nil {
			t.Fatalf("expected overwrite protection error, got nil")
		}
	})

	t.Run("allows existing target with overwrite", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "exists.txt")
		if err := os.WriteFile(path, []byte("data"), 0644); err != nil {
			t.Fatalf("WriteFile() error: %v", err)
		}
		if err := EnsureWritableFile(path, true); err != nil {
			t.Fatalf("EnsureWritableFile() unexpected error: %v", err)
		}
	})
}
