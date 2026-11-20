package actionkit

import (
	"strings"
	"testing"

	"github.com/dimmihq/dimmi/framework/ids"

	"github.com/dimmihq/dimmi/app/models"
)

func TestCallHeadersNameTheActionTheCallAndTheWorkspace(t *testing.T) {
	target := Target{
		Spec:   Spec{Name: "refund_invoice", SideEffect: SideEffectRead},
		CallID: ids.New(), WorkspaceID: ids.New(),
	}

	h := callHeaders(target)

	call := models.PublicID(models.PrefixAiActionCall, target.CallID)
	if got := h.Get("Dimmi-Action"); got != "refund_invoice" {
		t.Fatalf("Dimmi-Action = %q", got)
	}
	if got := h.Get("Dimmi-Action-Call"); got != call {
		t.Fatalf("Dimmi-Action-Call = %q", got)
	}
	if got := h.Get("Dimmi-Workspace"); got != models.PublicID(models.PrefixWorkspace, target.WorkspaceID) {
		t.Fatalf("Dimmi-Workspace = %q", got)
	}
	if got := h.Get("Idempotency-Key"); got != "" {
		t.Fatalf("a read carries Idempotency-Key %q", got)
	}

	target.Spec.SideEffect = SideEffectWrite
	if got := callHeaders(target).Get("Idempotency-Key"); got != call {
		t.Fatalf("Idempotency-Key = %q", got)
	}
}

func TestUserAgentLinksTheActionsPage(t *testing.T) {
	if !strings.Contains(userAgent, "(+https://trydimmi.com/docs/ai/actions/)") {
		t.Fatalf("userAgent = %q", userAgent)
	}
}
