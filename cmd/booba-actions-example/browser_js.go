//go:build js

package main

import "fmt"

func serveBrowser(string, string) error {
	return fmt.Errorf("-listen is available in the native build only")
}
