package analyzer

import (
	"fmt"
	"strings"
	"testing"
)

func TestParseSourceRejectsDuplicateActivityIDs(t *testing.T) {
	var source strings.Builder
	source.WriteString("graph id=graph release=release\n")
	for index, name := range RequiredActivities {
		id := fmt.Sprintf("activity-%d", index)
		if index == 1 {
			id = "activity-0"
		}
		fmt.Fprintf(&source, "activity id=%s activity=%s proof=FOUNDATION artifact=artifact authority=READ_ONLY\n", id, name)
	}
	_, err := ParseSource([]byte(source.String()))
	if err == nil || !strings.Contains(err.Error(), "duplicate activity id activity-0") {
		t.Fatalf("ParseSource duplicate activity ID error = %v", err)
	}
}
