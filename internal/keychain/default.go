// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package keychain

import "fmt"

// defaultKeychain implements KeychainAccess using the real platform keychain.
type defaultKeychain struct{}

func (d *defaultKeychain) Get(service, account string) (string, error) {
	val := Get(service, account)
	if val == "" {
		return "", fmt.Errorf("keychain entry not found: %s/%s", service, account)
	}
	return val, nil
}

func (d *defaultKeychain) Set(service, account, value string) error {
	return Set(service, account, value)
}

func (d *defaultKeychain) Remove(service, account string) error {
	return Remove(service, account)
}

// Default returns a KeychainAccess backed by the real platform keychain.
func Default() KeychainAccess {
	return &defaultKeychain{}
}
