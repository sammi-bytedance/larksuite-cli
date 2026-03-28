// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmdutil

import (
	"bytes"
	"net/http"
	"testing"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"

	"github.com/larksuite/cli/internal/core"
	"github.com/larksuite/cli/internal/httpmock"
)

// noopKeychain is a no-op KeychainAccess for tests that don't need keychain.
type noopKeychain struct{}

func (n *noopKeychain) Get(service, account string) (string, error) { return "", nil }
func (n *noopKeychain) Set(service, account, value string) error    { return nil }
func (n *noopKeychain) Remove(service, account string) error        { return nil }

// TestFactory creates a Factory for testing.
// Returns (factory, stdout buffer, stderr buffer, http mock registry).
func TestFactory(t *testing.T, config *core.CliConfig) (*Factory, *bytes.Buffer, *bytes.Buffer, *httpmock.Registry) {
	t.Helper()

	reg := &httpmock.Registry{}
	t.Cleanup(func() { reg.Verify(t) })

	stdoutBuf := &bytes.Buffer{}
	stderrBuf := &bytes.Buffer{}

	mockClient := httpmock.NewClient(reg)
	// SDK mock client wraps the mock transport with UserAgentTransport
	// so that User-Agent overrides the SDK default (oapi-sdk-go/v3.x.x).
	sdkMockClient := &http.Client{
		Transport: &UserAgentTransport{Base: reg},
	}

	// Build a test LarkClient using the config
	var testLarkClient *lark.Client
	if config != nil && config.AppID != "" {
		opts := []lark.ClientOptionFunc{
			lark.WithLogLevel(larkcore.LogLevelError),
			lark.WithHttpClient(sdkMockClient),
			lark.WithHeaders(BaseSecurityHeaders()),
		}
		if config.Brand != "" {
			opts = append(opts, lark.WithOpenBaseUrl(core.ResolveOpenBaseURL(config.Brand)))
		}
		testLarkClient = lark.NewClient(config.AppID, config.AppSecret, opts...)
	}

	f := &Factory{
		Config:     func() (*core.CliConfig, error) { return config, nil },
		AuthConfig: func() (*core.CliConfig, error) { return config, nil },
		HttpClient: func() (*http.Client, error) { return mockClient, nil },
		LarkClient: func() (*lark.Client, error) { return testLarkClient, nil },
		IOStreams:  &IOStreams{In: nil, Out: stdoutBuf, ErrOut: stderrBuf},
		Keychain:   &noopKeychain{},
	}
	return f, stdoutBuf, stderrBuf, reg
}
