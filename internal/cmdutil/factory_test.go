// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmdutil

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/larksuite/cli/internal/core"
)

// newCmdWithAsFlag creates a cobra.Command with a --as string flag for testing.
func newCmdWithAsFlag(asValue string, changed bool) *cobra.Command {
	cmd := &cobra.Command{Use: "test"}
	cmd.Flags().String("as", "auto", "identity")
	if changed {
		_ = cmd.Flags().Set("as", asValue)
	}
	return cmd
}

// --- ResolveAs tests ---

func TestResolveAs_ExplicitAs(t *testing.T) {
	f, _, _, _ := TestFactory(t, &core.CliConfig{AppID: "a", AppSecret: "s"})
	cmd := newCmdWithAsFlag("bot", true)

	got := f.ResolveAs(cmd, core.AsBot)
	if got != core.AsBot {
		t.Errorf("want bot, got %s", got)
	}
	if f.IdentityAutoDetected {
		t.Error("IdentityAutoDetected should be false for explicit --as")
	}
	if f.ResolvedIdentity != core.AsBot {
		t.Errorf("ResolvedIdentity want bot, got %s", f.ResolvedIdentity)
	}
}

func TestResolveAs_ExplicitAsUser(t *testing.T) {
	f, _, _, _ := TestFactory(t, &core.CliConfig{AppID: "a", AppSecret: "s"})
	cmd := newCmdWithAsFlag("user", true)

	got := f.ResolveAs(cmd, core.AsUser)
	if got != core.AsUser {
		t.Errorf("want user, got %s", got)
	}
	if f.ResolvedIdentity != core.AsUser {
		t.Errorf("ResolvedIdentity want user, got %s", f.ResolvedIdentity)
	}
}

func TestResolveAs_ExplicitAuto_FallsToAutoDetect(t *testing.T) {
	// --as auto explicitly: should fall through to auto-detect
	// Config has no UserOpenId → auto-detect returns bot
	f, _, _, _ := TestFactory(t, &core.CliConfig{AppID: "a", AppSecret: "s"})
	cmd := newCmdWithAsFlag("auto", true)

	got := f.ResolveAs(cmd, "auto")
	if got != core.AsBot {
		t.Errorf("want bot (auto-detect, no login), got %s", got)
	}
	if !f.IdentityAutoDetected {
		t.Error("IdentityAutoDetected should be true for auto-detect path")
	}
}

func TestResolveAs_DefaultAs_FromConfig(t *testing.T) {
	f, _, _, _ := TestFactory(t, &core.CliConfig{
		AppID: "a", AppSecret: "s",
		DefaultAs: "bot",
	})
	cmd := newCmdWithAsFlag("auto", false) // --as not changed

	got := f.ResolveAs(cmd, "auto")
	if got != core.AsBot {
		t.Errorf("want bot (from default-as config), got %s", got)
	}
	if f.IdentityAutoDetected {
		t.Error("IdentityAutoDetected should be false for default-as path")
	}
}

func TestResolveAs_DefaultAs_FromEnv(t *testing.T) {
	os.Setenv("LARKSUITE_CLI_DEFAULT_AS", "user")
	defer os.Unsetenv("LARKSUITE_CLI_DEFAULT_AS")

	f, _, _, _ := TestFactory(t, &core.CliConfig{AppID: "a", AppSecret: "s"})
	cmd := newCmdWithAsFlag("auto", false)

	got := f.ResolveAs(cmd, "auto")
	if got != core.AsUser {
		t.Errorf("want user (from env), got %s", got)
	}
}

func TestResolveAs_DefaultAs_AutoValue_FallsToAutoDetect(t *testing.T) {
	// default-as = "auto" should fall through to auto-detect
	f, _, _, _ := TestFactory(t, &core.CliConfig{
		AppID: "a", AppSecret: "s",
		DefaultAs: "auto",
	})
	cmd := newCmdWithAsFlag("auto", false)

	got := f.ResolveAs(cmd, "auto")
	// No UserOpenId → auto-detect returns bot
	if got != core.AsBot {
		t.Errorf("want bot (auto-detect), got %s", got)
	}
	if !f.IdentityAutoDetected {
		t.Error("IdentityAutoDetected should be true")
	}
}

func TestResolveAs_NilCmd_AutoDetect(t *testing.T) {
	f, _, _, _ := TestFactory(t, &core.CliConfig{AppID: "a", AppSecret: "s"})

	got := f.ResolveAs(nil, "auto")
	if got != core.AsBot {
		t.Errorf("want bot, got %s", got)
	}
}

// --- CheckIdentity tests ---

func TestCheckIdentity_Supported(t *testing.T) {
	f, _, _, _ := TestFactory(t, &core.CliConfig{AppID: "a", AppSecret: "s"})

	err := f.CheckIdentity(core.AsBot, []string{"bot", "user"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.ResolvedIdentity != core.AsBot {
		t.Errorf("ResolvedIdentity want bot, got %s", f.ResolvedIdentity)
	}
}

func TestCheckIdentity_Supported_UserOnly(t *testing.T) {
	f, _, _, _ := TestFactory(t, &core.CliConfig{AppID: "a", AppSecret: "s"})

	err := f.CheckIdentity(core.AsUser, []string{"user"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f.ResolvedIdentity != core.AsUser {
		t.Errorf("ResolvedIdentity want user, got %s", f.ResolvedIdentity)
	}
}

func TestCheckIdentity_Unsupported_Explicit(t *testing.T) {
	f, _, _, _ := TestFactory(t, &core.CliConfig{AppID: "a", AppSecret: "s"})
	f.IdentityAutoDetected = false // explicit --as

	err := f.CheckIdentity(core.AsUser, []string{"bot"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "--as user is not supported") {
		t.Errorf("unexpected error message: %v", err)
	}
	if !strings.Contains(err.Error(), "bot") {
		t.Errorf("error should mention supported identity: %v", err)
	}
}

func TestCheckIdentity_Unsupported_AutoDetected(t *testing.T) {
	f, _, _, _ := TestFactory(t, &core.CliConfig{AppID: "a", AppSecret: "s"})
	f.IdentityAutoDetected = true

	err := f.CheckIdentity(core.AsUser, []string{"bot"})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "resolved identity") {
		t.Errorf("expected 'resolved identity' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "hint: use --as bot") {
		t.Errorf("expected hint in error, got: %v", err)
	}
}

// --- ResolveConfig tests ---

func TestResolveConfig_Bot(t *testing.T) {
	cfg := &core.CliConfig{AppID: "a", AppSecret: "s"}
	f, _, _, _ := TestFactory(t, cfg)

	got, err := f.ResolveConfig(core.AsBot)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.AppID != "a" {
		t.Errorf("want AppID a, got %s", got.AppID)
	}
}

func TestResolveConfig_User(t *testing.T) {
	cfg := &core.CliConfig{AppID: "a", AppSecret: "s"}
	f, _, _, _ := TestFactory(t, cfg)

	got, err := f.ResolveConfig(core.AsUser)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.AppID != "a" {
		t.Errorf("want AppID a, got %s", got.AppID)
	}
}

// --- autoDetectIdentity tests ---

func TestAutoDetectIdentity_NoUserOpenId(t *testing.T) {
	f, _, _, _ := TestFactory(t, &core.CliConfig{AppID: "a", AppSecret: "s"})
	got := f.autoDetectIdentity()
	if got != core.AsBot {
		t.Errorf("want bot (no UserOpenId), got %s", got)
	}
}

func TestAutoDetectIdentity_ConfigError(t *testing.T) {
	f := &Factory{
		Config: func() (*core.CliConfig, error) {
			return nil, os.ErrNotExist
		},
	}
	got := f.autoDetectIdentity()
	if got != core.AsBot {
		t.Errorf("want bot (config error), got %s", got)
	}
}

// --- NewAPIClient / NewAPIClientWithConfig tests ---

func TestNewAPIClient(t *testing.T) {
	cfg := &core.CliConfig{AppID: "a", AppSecret: "s", Brand: core.BrandLark}
	f, _, _, _ := TestFactory(t, cfg)

	ac, err := f.NewAPIClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ac.Config.AppID != "a" {
		t.Errorf("want AppID a, got %s", ac.Config.AppID)
	}
}

func TestNewAPIClientWithConfig(t *testing.T) {
	cfg := &core.CliConfig{AppID: "a", AppSecret: "s", Brand: core.BrandLark}
	f, _, _, _ := TestFactory(t, cfg)

	ac, err := f.NewAPIClientWithConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ac.Config.AppID != "a" {
		t.Errorf("want AppID a, got %s", ac.Config.AppID)
	}
	if ac.SDK == nil {
		t.Error("SDK should not be nil")
	}
	if ac.HTTP == nil {
		t.Error("HTTP should not be nil")
	}
}

func TestNewAPIClientWithConfig_NilIOStreams(t *testing.T) {
	cfg := &core.CliConfig{AppID: "a", AppSecret: "s", Brand: core.BrandLark}
	f, _, _, _ := TestFactory(t, cfg)
	f.IOStreams = nil

	ac, err := f.NewAPIClientWithConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ac == nil {
		t.Fatal("expected non-nil APIClient")
	}
}
