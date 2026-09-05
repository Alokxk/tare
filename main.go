package main

import (
	"flag"
	"fmt"
	"os"
)

// Must stay a var: -ldflags "-X main.version=..." can only patch a string
// symbol, and a const would be inlined away and silently keep this default.
var version = "dev"

func main() {
	showVersion := flag.Bool("version", false, "print the Tare version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("tare", version)
		return
	}

	flag.Usage()
	os.Exit(2)
}
