package main

import (
	"context"
	"fmt"
	"os"

	"github.com/rannday/oaklog/internal/oaklog"
)

func main() {
	if err := oaklog.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
