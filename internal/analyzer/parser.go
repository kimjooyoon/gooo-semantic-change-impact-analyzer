package analyzer

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

func DigestBytes(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func ParseSource(raw []byte) (SourceDeclaration, error) {
	declaration := SourceDeclaration{Activities: []Activity{}}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	lineNumber := 0
	graphSeen := false
	seenActivities := map[string]bool{}
	seenActivityIDs := map[string]bool{}
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(stripComment(scanner.Text()))
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		values, err := keyValues(fields[1:])
		if err != nil {
			return SourceDeclaration{}, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		switch fields[0] {
		case "graph":
			if graphSeen || values["id"] == "" || values["release"] == "" {
				return SourceDeclaration{}, fmt.Errorf("line %d: invalid graph declaration", lineNumber)
			}
			declaration.GraphID = values["id"]
			declaration.Release = values["release"]
			graphSeen = true
		case "activity":
			activity := Activity{
				ID: values["id"], Name: values["activity"], Proof: values["proof"],
				Artifact: values["artifact"], Authority: values["authority"],
			}
			if activity.ID == "" || activity.Name == "" || activity.Proof == "" || activity.Artifact == "" || activity.Authority == "" {
				return SourceDeclaration{}, fmt.Errorf("line %d: incomplete activity", lineNumber)
			}
			if seenActivities[activity.Name] {
				return SourceDeclaration{}, fmt.Errorf("line %d: duplicate activity %s", lineNumber, activity.Name)
			}
			if seenActivityIDs[activity.ID] {
				return SourceDeclaration{}, fmt.Errorf("line %d: duplicate activity id %s", lineNumber, activity.ID)
			}
			seenActivities[activity.Name] = true
			seenActivityIDs[activity.ID] = true
			declaration.Activities = append(declaration.Activities, activity)
		default:
			return SourceDeclaration{}, fmt.Errorf("line %d: unsupported declaration %s", lineNumber, fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return SourceDeclaration{}, err
	}
	if !graphSeen || declaration.GraphID == "" || len(declaration.Activities) == 0 {
		return SourceDeclaration{}, errors.New("source declaration is incomplete")
	}
	if err := ValidateActivities(declaration.Activities); err != nil {
		return SourceDeclaration{}, err
	}
	return declaration, nil
}

func stripComment(line string) string {
	if index := strings.Index(line, "//"); index >= 0 {
		return line[:index]
	}
	return line
}

func keyValues(fields []string) (map[string]string, error) {
	values := make(map[string]string, len(fields))
	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, errors.New("invalid key/value")
		}
		if _, exists := values[parts[0]]; exists {
			return nil, fmt.Errorf("duplicate key %s", parts[0])
		}
		values[parts[0]] = strings.Trim(parts[1], "\"'")
	}
	return values, nil
}

func ValidateActivities(activities []Activity) error {
	if len(activities) != len(RequiredActivities) {
		return fmt.Errorf("expected exactly %d activities, got %d", len(RequiredActivities), len(activities))
	}
	seen := map[string]int{}
	for _, activity := range activities {
		seen[activity.Name]++
		if activity.Authority != "READ_ONLY" {
			return fmt.Errorf("activity %s is not read-only", activity.Name)
		}
	}
	for _, required := range RequiredActivities {
		if seen[required] != 1 {
			return fmt.Errorf("activity %s must bind exactly once", required)
		}
	}
	return nil
}

func LoadContract(path string) (Contract, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Contract{}, nil, err
	}
	var contract Contract
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contract); err != nil {
		return Contract{}, nil, err
	}
	if err := ValidateContract(contract); err != nil {
		return Contract{}, nil, err
	}
	return contract, raw, nil
}

func ValidateContract(contract Contract) error {
	if contract.Schema != ContractSchema || contract.Total != 8 || len(contract.Scenarios) != 8 {
		return errors.New("contract must contain exactly eight scenarios")
	}
	if len(contract.Activities) != len(RequiredActivities) {
		return errors.New("contract activity denominator is incomplete")
	}
	for index, required := range RequiredActivities {
		if contract.Activities[index] != required {
			return fmt.Errorf("activity ordinal %d is %s, want %s", index+1, contract.Activities[index], required)
		}
	}
	expected := []struct {
		id       string
		decision string
		unknown  string
	}{
		{"symbol-body-precise-dependent", DecisionClosed, ""},
		{"type-contract-transitive-dependent", DecisionClosed, ""},
		{"comment-only-zero-semantic-impact", DecisionClosed, ""},
		{"deterministic-replay", DecisionClosed, ""},
		{"missing-artifact-mapping", DecisionUnknown, UnknownDirect},
		{"stale-dependency-edge", DecisionUnknown, UnknownStale},
		{"omitted-known-dependent", DecisionRefuted, ""},
		{"unchanged-claim-semantic-digest-change", DecisionRefuted, ""},
	}
	for index, scenario := range contract.Scenarios {
		want := expected[index]
		if scenario.Ordinal != index+1 || scenario.ID != want.id || scenario.Expected != want.decision || scenario.UnknownClass != want.unknown {
			return fmt.Errorf("contract scenario %d does not match fixed v1 denominator", index+1)
		}
	}
	return nil
}

func LoadFixture(path string) (CaseFixture, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return CaseFixture{}, err
	}
	var fixture CaseFixture
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&fixture); err != nil {
		return CaseFixture{}, fmt.Errorf("decode %s: %w", path, err)
	}
	if err := ValidateFixture(fixture); err != nil {
		return CaseFixture{}, fmt.Errorf("fixture %s: %w", path, err)
	}
	return fixture, nil
}

func ValidateFixture(fixture CaseFixture) error {
	if fixture.Schema != "gooo/semantic-change-impact-analyzer/case/v1" || fixture.CaseID == "" || fixture.Description == "" {
		return errors.New("invalid fixture header")
	}
	if err := ValidateGraph(fixture.Before); err != nil {
		return fmt.Errorf("before graph: %w", err)
	}
	if err := ValidateGraph(fixture.After); err != nil {
		return fmt.Errorf("after graph: %w", err)
	}
	if fixture.Expected.Decision == "" {
		return errors.New("expected decision is required")
	}
	if fixture.Expected.ImpactedNodes < 0 || fixture.Expected.UnaffectedNodes < 0 || fixture.Expected.CausalEdges < 0 || fixture.Expected.MinimalFrontier < 0 || fixture.Expected.SelectedProofObligations < 0 {
		return errors.New("expected counts must be non-negative integers")
	}
	if fixture.Replay.Enabled && fixture.Replay.Runs < 2 {
		return errors.New("replay requires at least two runs")
	}
	return nil
}

func ValidateGraph(graph SemanticGraph) error {
	if graph.Schema != GraphSchema || graph.GraphID == "" || graph.ReleaseTag == "" || graph.ReleaseDigest == "" || graph.SemanticDigest == "" || len(graph.Nodes) == 0 {
		return errors.New("invalid semantic graph header")
	}
	nodeIDs := make(map[string]bool, len(graph.Nodes))
	for _, node := range graph.Nodes {
		if node.ID == "" || node.Kind == "" || node.SemanticDigest == "" || nodeIDs[node.ID] {
			return fmt.Errorf("invalid or duplicate node %s", node.ID)
		}
		nodeIDs[node.ID] = true
	}
	edgeIDs := make(map[string]bool, len(graph.Edges))
	for _, edge := range graph.Edges {
		if edge.EdgeID == "" || edge.From == "" || edge.To == "" || edge.Kind == "" || edge.ValueType == "" || edgeIDs[edge.EdgeID] {
			return fmt.Errorf("invalid or duplicate edge %s", edge.EdgeID)
		}
		edgeIDs[edge.EdgeID] = true
	}
	return nil
}

func CanonicalGraph(graph SemanticGraph) SemanticGraph {
	copyGraph := graph
	copyGraph.Nodes = append([]SemanticNode(nil), graph.Nodes...)
	copyGraph.Edges = append([]TypedDependencyEdge(nil), graph.Edges...)
	sort.Slice(copyGraph.Nodes, func(i, j int) bool { return copyGraph.Nodes[i].ID < copyGraph.Nodes[j].ID })
	sort.Slice(copyGraph.Edges, func(i, j int) bool { return copyGraph.Edges[i].EdgeID < copyGraph.Edges[j].EdgeID })
	return copyGraph
}

func GraphFingerprint(graph SemanticGraph) string {
	canonical := CanonicalGraph(graph)
	raw, err := json.Marshal(canonical)
	if err != nil {
		return ""
	}
	return DigestBytes(raw)
}
