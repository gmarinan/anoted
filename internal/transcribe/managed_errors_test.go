package transcribe

import (
	"errors"
	"testing"
)

func TestFormatCmdError_Generic(t *testing.T) {
	err := formatCmdError("pip", errors.New("boom"))
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got == "" || got == "boom" {
		t.Fatalf("unexpected message: %q", got)
	}
}
