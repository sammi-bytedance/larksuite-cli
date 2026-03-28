// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package cmdutil

import "io"

// IOStreams provides the standard input/output/error streams.
// Commands should use these instead of os.Stdin/Stdout/Stderr
// to enable testing and output capture.
type IOStreams struct {
	In         io.Reader
	Out        io.Writer
	ErrOut     io.Writer
	IsTerminal bool
}
