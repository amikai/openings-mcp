// Package openingsmcp adapts the internal job-board clients into MCP tools.
//
// Handlers report failures by returning a plain error: the SDK turns it into
// an IsError tool result with the message as text and no structured content.
package openingsmcp
