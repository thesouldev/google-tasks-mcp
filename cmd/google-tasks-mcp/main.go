package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/thesouldev/google-tasks-mcp/internal/auth"
	"github.com/thesouldev/google-tasks-mcp/internal/server"
)

var version = "dev"

const usage = `google-tasks-mcp %s

Usage:
  google-tasks-mcp           start the MCP server on stdio
  google-tasks-mcp auth      sign in to Google in the browser and save the token
  google-tasks-mcp logout    delete the saved token
  google-tasks-mcp version   print the version
`

func main() {
	if version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			version = info.Main.Version
		}
	}

	cmd := ""
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	var err error
	switch cmd {
	case "", "serve":
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		err = server.New(version, server.Options{}).Run(ctx)
	case "auth", "login":
		err = auth.RunAuthFlow()
	case "logout":
		err = auth.Logout()
	case "version", "--version", "-v":
		fmt.Println(version)
	case "help", "--help", "-h":
		fmt.Printf(usage, version)
	default:
		fmt.Fprintf(os.Stderr, usage, version)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
