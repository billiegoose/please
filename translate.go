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
	Interactive bool   `json:"interactive"`
}

var systemPrompt = `You are a CLI command translator. Given English input, return a JSON array of CLI command interpretations.

Rules:
- Each item has "cmd" (the shell command), "explanation" (terse, ≤10 words), and "interactive" (bool — true if the command opens an interactive UI, editor, pager, or requires user input, e.g. vim, git commit, ssh, less, top)
- Return only the JSON array, no other text
- If input is empty or nonsensical, return []
- ONLY return multiple options if the request is genuinely ambiguous — i.e. different interpretations of intent would produce meaningfully different outcomes. Do NOT return multiple options just because there are several commands that achieve the same result. When in doubt, return one option.
- Good reason for multiple options: "delete old folders" → one option uses mtime, another uses ctime — genuinely different files could be affected
- Bad reason for multiple options: offering "find -delete" vs "find -exec rm -rf" when both delete the same files

Example:
Input: "list all files including hidden ones"
Output: [{"cmd":"ls -la","explanation":"list all files with details","interactive":false}]`

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
