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

func TestValidateGraphRejectsUnknownEdgeEndpoint(t *testing.T) {
	graph := validTestGraph()
	graph.Edges[0].To = "missing"
	if err := ValidateGraph(graph); err == nil {
		t.Fatal("unknown edge endpoint was accepted")
	}
}

func TestValidateGraphRejectsEndpointDigestMismatch(t *testing.T) {
	graph := validTestGraph()
	graph.Edges[0].FromDigest = "sha256:wrong"
	if err := ValidateGraph(graph); err == nil {
		t.Fatal("endpoint digest mismatch was accepted")
	}
}

func validTestGraph() SemanticGraph {
	return SemanticGraph{
		Schema:         GraphSchema,
		GraphID:        "test-graph",
		ReleaseTag:     "v1.0.0",
		ReleaseDigest:  "sha256:release",
		SemanticDigest: "sha256:graph",
		Nodes: []SemanticNode{
			{ID: "a", Kind: "symbol", SemanticDigest: "sha256:a"},
			{ID: "b", Kind: "artifact", SemanticDigest: "sha256:b"},
		},
		Edges: []TypedDependencyEdge{
			{EdgeID: "a-b", From: "a", To: "b", Kind: "GENERATES", ValueType: "symbol->artifact", FromDigest: "sha256:a", ToDigest: "sha256:b"},
		},
	}
}
