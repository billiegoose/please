package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
)

type Suggestion struct {
	Cmd         string `json:"cmd"`
	Explanation string `json:"explanation"`
}

var systemPrompt = `You are a CLI command translator. Given English input, return 1-3 CLI command interpretations as a JSON array.

Rules:
- Each item has "cmd" (the shell command) and "explanation" (terse, ≤10 words)
- Return only the JSON array, no other text
- Order by most likely interpretation first
- If input is empty or nonsensical, return []

Example:
Input: "list all files including hidden ones"
Output: [{"cmd":"ls -la","explanation":"list all files with details"},{"cmd":"ls -A","explanation":"list all files, no . and .."}]`

func translate(ctx context.Context, client *anthropic.Client, input string) ([]Suggestion, error) {
	if input == "" {
		return nil, nil
	}

	resp, err := client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 512,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(input)),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("API error: %w", err)
	}

	var text string
	for _, block := range resp.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			text = tb.Text
			break
		}
	}

	// Strip markdown code fences if the model wrapped the JSON
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "```") {
		if i := strings.Index(text, "\n"); i != -1 {
			text = text[i+1:]
		}
		text = strings.TrimSuffix(text, "```")
		text = strings.TrimSpace(text)
	}

	var suggestions []Suggestion
	if err := json.Unmarshal([]byte(text), &suggestions); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	return suggestions, nil
}
