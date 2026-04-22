//go:build !js

package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/NimbleMarkets/go-booba/internal/sipclient"
)

func main() {
	err := sipclient.Execute(context.Background())
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, "Error:", err)
	switch {
	case errors.Is(err, sipclient.ErrProtocol):
		os.Exit(2)
	case errors.Is(err, sipclient.ErrTransport):
		os.Exit(3)
	case errors.Is(err, sipclient.ErrConnect):
		os.Exit(1)
	default:
		os.Exit(1)
	}
}
