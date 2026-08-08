//go:build !windows

package cli

// Unix terminals interpret ANSI escapes and speak UTF-8 without being
// asked; there is nothing to set up.
func setupConsole() func() { return func() {} }
