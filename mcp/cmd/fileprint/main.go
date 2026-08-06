// SPDX-FileCopyrightText: 2026 Robert Bosch GmbH
//
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	pathPtr := flag.String("path", "", "Path of the file to print to stdout")
	flag.Parse()
	if *pathPtr == "" {
		fmt.Fprintln(os.Stderr, "missing required -path argument")
		os.Exit(1)
	}

	data, err := os.ReadFile(*pathPtr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read file %q: %v\n", *pathPtr, err)
		os.Exit(1)
	}
	fmt.Print(string(data))
	fmt.Fprintln(os.Stderr, "Fileprint completed successfully.")
}
