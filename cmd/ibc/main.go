package main

import (
	"fmt"
	"os"

	"github.com/gjermundgaraba/libibc/cmd/ibc/cmd"
	"github.com/pkg/errors"
)

type stackTracer interface {
	StackTrace() errors.StackTrace
}

func main() {

	rootCmd := cmd.NewRootCmd()
	if err := rootCmd.Execute(); err != nil {
		os.Stderr.WriteString("Something went wrong:\n")
		if err, ok := err.(stackTracer); ok {
			for _, f := range err.StackTrace() {
				os.Stderr.WriteString(fmt.Sprintf("%+s:%d\n", f, f))
			}
		}

		os.Exit(1)
	}
}

