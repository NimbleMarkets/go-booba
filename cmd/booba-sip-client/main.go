//go:build !js

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/NimbleMarkets/go-booba/internal/sipclient"
)

func main() {
	if err := sipclient.Execute(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
