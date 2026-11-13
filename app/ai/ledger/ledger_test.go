package ledger_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/dimmihq/dimmi/app/ai/ledger"
	"github.com/dimmihq/dimmi/app/contracts"
)

func text(s string) []contracts.ContentPart {
	return []contracts.ContentPart{{Text: s}}
}

func TestOfOrdersThePartsAsTheProviderReadsThem(t *testing.T) {
	req := contracts.ChatRequest{
		Tools:  []contracts.ToolSpec{{Name: "load_tools", Description: "d", Schema: json.RawMessage(`{}`)}},
		System: []contracts.SystemBlock{{Text: strings.Repeat("s", 100)}},
		Messages: []contracts.ChatMessage{
			{Role: contracts.ChatRoleUser, Parts: text("old question")},
			{Role: contracts.ChatRoleAssistant, Parts: text("old answer")},
			{Role: contracts.ChatRoleUser, Parts: text("new question")},
			{Role: contracts.ChatRoleAssistant, ToolCalls: []contracts.ToolCall{
				{ID: "1", Name: "load_tools", Arguments: json.RawMessage(`{}`)},
				{ID: "2", Name: "search_knowledge", Arguments: json.RawMessage(`{}`)},
			}},
			{Role: contracts.ChatRoleTool, ToolCallID: "1", Parts: text("definitions")},
			{Role: contracts.ChatRoleTool, ToolCallID: "2", Parts: text("passages")},
		},
	}
	var parts []string
	for _, s := range ledger.Of(req, 2) {
		parts = append(parts, s.Part)
	}
	want := "fixed,history,newest,tool_calls,loaded_tools,search"
	if strings.Join(parts, ",") != want {
		t.Fatalf("parts = %v", parts)
	}
}

func TestSplitFillsTheCacheFromTheStart(t *testing.T) {
	segments := []ledger.Segment{
		{Part: ledger.Fixed, Chars: 600},
		{Part: ledger.History, Chars: 300},
		{Part: ledger.Newest, Chars: 100},
	}
	got := ledger.Split(segments, contracts.LLMUsage{InputTokens: 100, CachedInputTokens: 700, CacheWriteTokens: 200})
	if got[ledger.Fixed] != (ledger.Tokens{Cached: 600}) {
		t.Fatalf("fixed = %+v", got[ledger.Fixed])
	}
	if got[ledger.History] != (ledger.Tokens{Cached: 100, Written: 200}) {
		t.Fatalf("history = %+v", got[ledger.History])
	}
	if got[ledger.Newest] != (ledger.Tokens{Fresh: 100}) {
		t.Fatalf("newest = %+v", got[ledger.Newest])
	}
	var sum int64
	for _, v := range got {
		sum += v.Total()
	}
	if sum != 1000 {
		t.Fatalf("sum = %d", sum)
	}
}
