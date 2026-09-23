package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSymbolBodyPreciseDependent(t *testing.T) { runFixtureTest(t, "symbol-body-precise-dependent") }
func TestTypeContractTransitiveDependent(t *testing.T) {
	runFixtureTest(t, "type-contract-transitive-dependent")
}
func TestCommentOnlyZeroSemanticImpact(t *testing.T) {
	runFixtureTest(t, "comment-only-zero-semantic-impact")
}
func TestDeterministicReplay(t *testing.T)    { runFixtureTest(t, "deterministic-replay") }
func TestMissingArtifactMapping(t *testing.T) { runFixtureTest(t, "missing-artifact-mapping") }
func TestStaleDependencyEdge(t *testing.T)    { runFixtureTest(t, "stale-dependency-edge") }
func TestOmittedKnownDependent(t *testing.T)  { runFixtureTest(t, "omitted-known-dependent") }
func TestUnchangedClaimDespiteSemanticDigestChange(t *testing.T) {
	runFixtureTest(t, "unchanged-claim-semantic-digest-change")
}

func TestStripCommentPreservesQuotedCommentMarker(t *testing.T) {
	line := `activity artifact="https://example.test/v1" // trailing comment`
	if got, want := stripComment(line), `activity artifact="https://example.test/v1" `; got != want {
		t.Fatalf("stripComment(%q)=%q want %q", line, got, want)
	}
}

func runFixtureTest(t *testing.T, caseID string) {
	t.Helper()
	root := filepath.Clean(filepath.Join("..", ".."))
	sourcePath := filepath.Join(root, "examples", "semantic-change-impact-analyzer-v1", "main.gooo")
	contractPath := filepath.Join(root, "contracts", "impact-denominator-v1.json")
	fixturePath := filepath.Join(root, "fixtures", "cases", caseID+".json")
	sourceRaw, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	contract, contractRaw, err := LoadContract(contractPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParseSource(sourceRaw); err != nil {
		t.Fatal(err)
	}
	fixture, err := LoadFixture(fixturePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, expected, err := contractScenario(contract, caseID); err != nil || expected != fixture.Expected.Decision {
		t.Fatalf("fixture is not bound to the fixed contract: %v", err)
	}
	result, err := AnalyzeFixture(fixture, sourceRaw, contractRaw, RuntimeOptions{Tests: TestMetrics{Total: 8, Selected: 8, Executed: 8}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Receipt.Decision != fixture.Expected.Decision {
		t.Fatalf("decision=%s want=%s", result.Receipt.Decision, fixture.Expected.Decision)
	}
}
