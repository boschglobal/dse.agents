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
	msgPtr := flag.String("msg", "Hello World!", "The message to display")
	flag.Parse()
	fmt.Printf("Processing message payload: %s\n", *msgPtr)
	fmt.Fprintln(os.Stderr, "Message completed successfully.")
}
