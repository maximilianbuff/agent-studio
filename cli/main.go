package main

import "github.com/maximilianbuff/agent-studio/cmd"

// version is set at build time via -ldflags "-X main.version=<tag>".
var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
