// Command organizze-mcp-oauth is the multi-tenant variant of organizze-mcp.
// It hosts an OAuth 2.1 Authorization Server and serves the same MCP toolset
// over Streamable HTTP, resolving each caller's Organizze credentials from
// the validated bearer token instead of process-wide env vars.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "organizze-mcp-oauth:", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("organizze-mcp-oauth: scaffolding — wire-up follows in later tasks")
	return nil
}
