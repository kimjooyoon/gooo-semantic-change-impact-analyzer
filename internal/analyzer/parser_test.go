package analyzer

import "testing"

func TestParseSourceRejectsUnknownDeclarationKey(t *testing.T) {
	raw := []byte("graph id=semantic-change release=v1 extra=ignored\n")
	if _, err := ParseSource(raw); err == nil {
		t.Fatal("unknown graph declaration key was accepted")
	}
}
