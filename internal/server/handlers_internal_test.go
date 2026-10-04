package server

import (
	"strings"
	"testing"
)

func TestContentDisposition(t *testing.T) {
	if got := contentDisposition("spec.pdf"); got != `attachment; filename="spec.pdf"` {
		t.Errorf("ascii = %q", got)
	}

	got := contentDisposition("报告 2026.pdf")
	if !strings.Contains(got, `filename="__ 2026.pdf"`) {
		t.Errorf("fallback missing: %q", got)
	}
	if !strings.Contains(got, `filename*=UTF-8''%E6%8A%A5%E5%91%8A%202026.pdf`) {
		t.Errorf("rfc5987 missing: %q", got)
	}

	// Quotes and control characters must not break the header.
	got = contentDisposition("we\"ird\n.pdf")
	if strings.Contains(got, "\n") || !strings.Contains(got, `filename="we_ird_.pdf"`) {
		t.Errorf("unsafe chars not sanitized: %q", got)
	}
}
