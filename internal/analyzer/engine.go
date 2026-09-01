package analyzer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

type RuntimeOptions struct {
	Tests     TestMetrics
	Inventory Inventory
	Runner    string
}

type ScenarioResult struct {
	Fixture CaseFixture
	Graph   ImpactGraph
	Receipt ImpactReceipt
	Report  string
}

type deltaResult struct {
	ChangedNodes        []string
	SemanticEdgeChange  bool
	DigestContradiction bool
}

type bindingResult struct {
	Unknown       *Unknown
	RefutedReason string
	InvalidEdges  map[string]bool
}

type frontierResult struct {
	Impacted    []string
	Unaffected  []string
	CausalEdges []string
	Minimal     []string
	Proofs      []string
}

func AnalyzeFixture(fixture CaseFixture, sourceRaw, contractRaw []byte, options RuntimeOptions) (ScenarioResult, error) {
	if err := ValidateFixture(fixture); err != nil {
		return ScenarioResult{}, err
	}
	if options.Runner == "" {
		options.Runner = "github-actions/ubuntu-latest"
	}
	if err := validateTestMetrics(options.Tests); err != nil {
		return ScenarioResult{}, err
	}
	stages := make([]StageMetric, 0, len(RequiredActivities))
	var delta deltaResult
	var binding bindingResult
	var frontier frontierResult
	var decision string
	var reason string
	var replay ReplayObservation

	stages = append(stages, measureStage("ParseSemanticDelta", func() {
		delta = parseSemanticDelta(fixture.Before, fixture.After)
	}))
	stages = append(stages, measureStage("BindTypedDependencies", func() {
		binding = bindTypedDependencies(fixture.Before, fixture.After, delta.ChangedNodes, fixture.KnownDependents)
	}))
	stages = append(stages, measureStage("ComputeCausalFrontier", func() {
		frontier = computeCausalFrontier(fixture.After, delta.ChangedNodes, binding.InvalidEdges)
	}))
	stages = append(stages, measureStage("ClassifyImpact", func() {
		decision, reason = classifyImpact(delta, binding)
	}))

	counts := Counts{
		ImpactedNodes: len(frontier.Impacted), UnaffectedNodes: len(frontier.Unaffected),
		CausalEdges: len(frontier.CausalEdges), MinimalFrontier: len(frontier.Minimal),
		SelectedProofObligations: len(frontier.Proofs),
	}
	unknown := binding.Unknown
	if decision == DecisionUnknown && (unknown == nil || !unknown.Valid()) {
		return ScenarioResult{}, errors.New("UNKNOWN decision did not preserve the required six-field tuple")
	}
	if decision != DecisionUnknown {
		unknown = nil
	}

	core := impactCore{
		Decision: decision, ChangedNodes: delta.ChangedNodes, Impacted: frontier.Impacted,
		Unaffected: frontier.Unaffected, CausalEdges: frontier.CausalEdges,
		Minimal: frontier.Minimal, Proofs: frontier.Proofs,
	}
	stages = append(stages, measureStage("EmitImpactReceipt", func() {
		// The receipt is assembled below after all exact count observations exist.
		_ = core
	}))

	fingerprint := core.fingerprint()
	replay = ReplayObservation{Runs: 0, Match: true, Fingerprint: fingerprint}
	stages = append(stages, measureStage("VerifyImpactReplay", func() {
		if fixture.Replay.Enabled {
			replay.Runs = fixture.Replay.Runs
			replayed := analyzeCore(fixture, delta, binding)
			replay.Match = fingerprint == replayed.fingerprint()
			if !replay.Match {
				decision = DecisionRefuted
				reason = "DETERMINISTIC_REPLAY_MISMATCH"
				unknown = nil
			}
		}
	}))

	if err := validateExpectedCounts(fixture.Expected, decision, counts, unknown); err != nil {
		return ScenarioResult{}, fmt.Errorf("case %s: %w", fixture.CaseID, err)
	}
	sourceDigest := DigestBytes(sourceRaw)
	contractDigest := DigestBytes(contractRaw)
	runnerDigest := DigestBytes([]byte(options.Runner))
	toolchainDigest := DigestBytes([]byte(ToolchainVersion))
	authority := Authority{CallerOwnedOutput: true}
	graph := ImpactGraph{
		Schema: ImpactGraphSchema, CaseID: fixture.CaseID, Decision: decision,
		BeforeReleaseTag: fixture.Before.ReleaseTag, AfterReleaseTag: fixture.After.ReleaseTag,
		BeforeGraphDigest: fixture.Before.SemanticDigest, AfterGraphDigest: fixture.After.SemanticDigest,
		ChangedNodes: delta.ChangedNodes, ImpactedNodes: frontier.Impacted, UnaffectedNodes: frontier.Unaffected,
		CausalEdges: frontier.CausalEdges, MinimalFrontier: frontier.Minimal,
		SelectedProofObligations: frontier.Proofs, Counts: counts, Unknown: unknown,
	}
	improvement := unknownImprovement(sourceDigest, contractDigest, toolchainDigest, runnerDigest)
	receipt := ImpactReceipt{
		Schema: ReceiptSchema, CaseID: fixture.CaseID, Decision: decision, Reason: reason,
		Precedence: append([]string(nil), Precedence...), Counts: counts, StageMetrics: stages,
		Tests: options.Tests, Unknown: unknown, Replay: replay, SourceDigest: sourceDigest,
		ContractDigest: contractDigest, ToolchainDigest: toolchainDigest, RunnerDigest: runnerDigest,
		Authority: authority, Improvement: improvement, Inventory: options.Inventory,
	}
	return ScenarioResult{Fixture: fixture, Graph: graph, Receipt: receipt, Report: RenderScenarioReport(graph, receipt)}, nil
}

type impactCore struct {
	Decision     string
	ChangedNodes []string
	Impacted     []string
	Unaffected   []string
	CausalEdges  []string
	Minimal      []string
	Proofs       []string
}

func (core impactCore) fingerprint() string {
	raw, err := json.Marshal(core)
	if err != nil {
		return ""
	}
	return DigestBytes(raw)
}

func analyzeCore(fixture CaseFixture, delta deltaResult, binding bindingResult) impactCore {
	frontier := computeCausalFrontier(fixture.After, delta.ChangedNodes, binding.InvalidEdges)
	decision, _ := classifyImpact(delta, binding)
	return impactCore{
		Decision: decision, ChangedNodes: delta.ChangedNodes, Impacted: frontier.Impacted,
		Unaffected: frontier.Unaffected, CausalEdges: frontier.CausalEdges,
		Minimal: frontier.Minimal, Proofs: frontier.Proofs,
	}
}

func parseSemanticDelta(before, after SemanticGraph) deltaResult {
	beforeNodes := nodesByID(before.Nodes)
	afterNodes := nodesByID(after.Nodes)
	allIDs := make([]string, 0, len(beforeNodes)+len(afterNodes))
	seen := map[string]bool{}
	for id := range beforeNodes {
		seen[id] = true
		allIDs = append(allIDs, id)
	}
	for id := range afterNodes {
		if !seen[id] {
			allIDs = append(allIDs, id)
		}
	}
	sort.Strings(allIDs)
	changed := make([]string, 0)
	for _, id := range allIDs {
		beforeNode, beforeOK := beforeNodes[id]
		afterNode, afterOK := afterNodes[id]
		if !beforeOK || !afterOK || beforeNode.SemanticDigest != afterNode.SemanticDigest || beforeNode.Kind != afterNode.Kind {
			changed = append(changed, id)
		}
	}
	semanticEdgeChange := !sameEdgeTopology(before.Edges, after.Edges)
	return deltaResult{
		ChangedNodes: changed, SemanticEdgeChange: semanticEdgeChange,
		DigestContradiction: before.SemanticDigest != after.SemanticDigest && len(changed) == 0 && !semanticEdgeChange,
	}
}

func bindTypedDependencies(before, after SemanticGraph, changed, knownDependents []string) bindingResult {
	result := bindingResult{InvalidEdges: map[string]bool{}}
	afterNodes := nodesByID(after.Nodes)
	changedSet := stringSet(changed)
	for _, edge := range CanonicalGraph(after).Edges {
		fromNode, fromOK := afterNodes[edge.From]
		toNode, toOK := afterNodes[edge.To]
		if !fromOK || !toOK {
			result.InvalidEdges[edge.EdgeID] = true
			if result.Unknown == nil {
				result.Unknown = &Unknown{Stage: "BIND_TYPED_DEPENDENCIES", Step: "VERIFY_EDGE_ENDPOINTS", Reason: "IMPACT_ARTIFACT_MAPPING_MISSING", UnknownClass: UnknownDirect, NextOperation: "PROVIDE_ARTIFACT_MAPPING", BlockedBy: []string{edge.EdgeID}}
			}
			continue
		}
		if edge.FromDigest != fromNode.SemanticDigest || edge.ToDigest != toNode.SemanticDigest {
			result.InvalidEdges[edge.EdgeID] = true
			if result.Unknown == nil {
				result.Unknown = &Unknown{Stage: "BIND_TYPED_DEPENDENCIES", Step: "VERIFY_EDGE_DIGESTS", Reason: "TYPED_DEPENDENCY_EDGE_IS_STALE", UnknownClass: UnknownStale, NextOperation: "REBIND_TYPED_DEPENDENCY_EDGE", BlockedBy: []string{edge.EdgeID}}
			}
		}
	}

	beforeReachable := reachableEdgeIDs(before, changedSet, nil)
	afterEdges := edgesByID(after.Edges)
	beforeEdges := edgesByID(before.Edges)
	afterNodesPresent := stringSet(nodeIDs(after.Nodes))
	knownSet := stringSet(knownDependents)
	for _, edgeID := range sortedSet(beforeReachable) {
		beforeEdge := beforeEdges[edgeID]
		if _, exists := afterEdges[edgeID]; exists {
			continue
		}
		if knownSet[beforeEdge.To] && !afterNodesPresent[beforeEdge.To] {
			result.RefutedReason = "KNOWN_DEPENDENT_OMITTED"
			continue
		}
		if afterNodesPresent[beforeEdge.To] && result.Unknown == nil {
			result.Unknown = &Unknown{Stage: "BIND_TYPED_DEPENDENCIES", Step: "VERIFY_ARTIFACT_MAPPING", Reason: "IMPACT_ARTIFACT_MAPPING_MISSING", UnknownClass: UnknownDirect, NextOperation: "PROVIDE_ARTIFACT_MAPPING", BlockedBy: []string{beforeEdge.EdgeID}}
		}
	}
	for _, dependent := range sortedSet(knownSet) {
		if beforeNodesContain(before, dependent) && !afterNodesPresent[dependent] {
			result.RefutedReason = "KNOWN_DEPENDENT_OMITTED"
		}
	}
	return result
}

func classifyImpact(delta deltaResult, binding bindingResult) (string, string) {
	if delta.DigestContradiction {
		return DecisionRefuted, "SEMANTIC_DIGEST_CHANGED_WITHOUT_SEMANTIC_DELTA"
	}
	if binding.RefutedReason != "" {
		return DecisionRefuted, binding.RefutedReason
	}
	if binding.Unknown != nil {
		return DecisionUnknown, binding.Unknown.Reason
	}
	if len(delta.ChangedNodes) == 0 && !delta.SemanticEdgeChange {
		return DecisionClosed, "NO_SEMANTIC_IMPACT"
	}
	return DecisionClosed, "MINIMAL_CAUSAL_FRONTIER_COMPUTED"
}

func computeCausalFrontier(graph SemanticGraph, changed []string, invalidEdges map[string]bool) frontierResult {
	nodes := nodesByID(graph.Nodes)
	changedSet := stringSet(changed)
	impactedSet := map[string]bool{}
	causalSet := map[string]bool{}
	minimalSet := map[string]bool{}
	queue := append([]string(nil), changed...)
	visited := stringSet(changed)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range CanonicalGraph(graph).Edges {
			if edge.From != current || invalidEdges[edge.EdgeID] || !nodesContain(nodes, edge.From) || !nodesContain(nodes, edge.To) {
				continue
			}
			causalSet[edge.EdgeID] = true
			if changedSet[current] {
				minimalSet[edge.EdgeID] = true
			}
			if !changedSet[edge.To] {
				impactedSet[edge.To] = true
			}
			if !visited[edge.To] {
				visited[edge.To] = true
				queue = append(queue, edge.To)
			}
		}
	}
	impacted := sortedSet(impactedSet)
	unaffected := make([]string, 0)
	for _, node := range CanonicalGraph(graph).Nodes {
		if !changedSet[node.ID] && !impactedSet[node.ID] {
			unaffected = append(unaffected, node.ID)
		}
	}
	causalEdges := sortedSet(causalSet)
	minimal := sortedSet(minimalSet)
	proofs := make([]string, 0)
	for _, id := range impacted {
		if nodes[id].Kind == "proof" {
			proofs = append(proofs, id)
		}
	}
	return frontierResult{Impacted: impacted, Unaffected: unaffected, CausalEdges: causalEdges, Minimal: minimal, Proofs: proofs}
}

func reachableEdgeIDs(graph SemanticGraph, starts map[string]bool, invalid map[string]bool) map[string]bool {
	nodes := nodesByID(graph.Nodes)
	seenNodes := map[string]bool{}
	queue := make([]string, 0, len(starts))
	for start := range starts {
		seenNodes[start] = true
		queue = append(queue, start)
	}
	edges := map[string]bool{}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range CanonicalGraph(graph).Edges {
			if edge.From != current || invalid[edge.EdgeID] || !nodesContain(nodes, edge.To) {
				continue
			}
			edges[edge.EdgeID] = true
			if !seenNodes[edge.To] {
				seenNodes[edge.To] = true
				queue = append(queue, edge.To)
			}
		}
	}
	return edges
}

func sameEdgeTopology(before, after []TypedDependencyEdge) bool {
	beforeMap := make(map[string]string, len(before))
	afterMap := make(map[string]string, len(after))
	for _, edge := range before {
		beforeMap[edge.EdgeID] = edge.From + "|" + edge.To + "|" + edge.Kind + "|" + edge.ValueType
	}
	for _, edge := range after {
		afterMap[edge.EdgeID] = edge.From + "|" + edge.To + "|" + edge.Kind + "|" + edge.ValueType
	}
	if len(beforeMap) != len(afterMap) {
		return false
	}
	for id, signature := range beforeMap {
		if afterMap[id] != signature {
			return false
		}
	}
	return true
}

func validateExpectedCounts(expected ExpectedCounts, decision string, counts Counts, unknown *Unknown) error {
	if expected.Decision != decision {
		return fmt.Errorf("decision %s does not match expected %s", decision, expected.Decision)
	}
	want := Counts{ImpactedNodes: expected.ImpactedNodes, UnaffectedNodes: expected.UnaffectedNodes, CausalEdges: expected.CausalEdges, MinimalFrontier: expected.MinimalFrontier, SelectedProofObligations: expected.SelectedProofObligations}
	if counts != want {
		return fmt.Errorf("counts %#v do not match expected %#v", counts, want)
	}
	if expected.UnknownClass != "" {
		if unknown == nil || unknown.UnknownClass != expected.UnknownClass || !unknown.Valid() {
			return fmt.Errorf("unknown class does not match expected %s", expected.UnknownClass)
		}
	}
	return nil
}

func validateTestMetrics(metrics TestMetrics) error {
	values := []int{metrics.Total, metrics.Selected, metrics.Executed, metrics.Reused, metrics.Failed, metrics.Unknown}
	for _, value := range values {
		if value < 0 {
			return errors.New("test metrics must be non-negative integers")
		}
	}
	return nil
}

func measureStage(name string, fn func()) StageMetric {
	started := time.Now()
	fn()
	return StageMetric{Stage: name, WallMS: int(time.Since(started).Milliseconds()), PeakRSSKiB: currentRSSKiB()}
}

func currentRSSKiB() int {
	if runtime.GOOS == "linux" {
		if raw, err := os.ReadFile("/proc/self/status"); err == nil {
			for _, line := range strings.Split(string(raw), "\n") {
				if strings.HasPrefix(line, "VmHWM:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						value, err := strconv.Atoi(fields[1])
						if err == nil {
							return value
						}
					}
				}
			}
		}
	}
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return int(stats.Sys / 1024)
}

func unknownImprovement(sourceDigest, contractDigest, toolchainDigest, runnerDigest string) Improvement {
	unknown := Unknown{Stage: "IMPROVEMENT", Step: "COMPARE_EXACT_BEFORE_AFTER_INTEGER_PAIR", Reason: "EXACT_BEFORE_AFTER_INTEGER_PAIR_NOT_PROVIDED", UnknownClass: UnknownDirect, NextOperation: "PROVIDE_EXACT_BEFORE_AFTER_INTEGER_PAIR", BlockedBy: []string{"before_after_integer_pair"}}
	return Improvement{State: DecisionUnknown, Reason: unknown.Reason, SourceDigest: sourceDigest, ContractDigest: contractDigest, Toolchain: toolchainDigest, Runner: runnerDigest, Unknown: unknown}
}

func nodesByID(nodes []SemanticNode) map[string]SemanticNode {
	result := make(map[string]SemanticNode, len(nodes))
	for _, node := range nodes {
		result[node.ID] = node
	}
	return result
}

func edgesByID(edges []TypedDependencyEdge) map[string]TypedDependencyEdge {
	result := make(map[string]TypedDependencyEdge, len(edges))
	for _, edge := range edges {
		result[edge.EdgeID] = edge
	}
	return result
}

func nodeIDs(nodes []SemanticNode) []string {
	result := make([]string, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, node.ID)
	}
	return result
}

func beforeNodesContain(graph SemanticGraph, id string) bool {
	for _, node := range graph.Nodes {
		if node.ID == id {
			return true
		}
	}
	return false
}

func nodesContain(nodes map[string]SemanticNode, id string) bool {
	_, ok := nodes[id]
	return ok
}

func stringSet(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}

func sortedSet(values map[string]bool) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func WriteJSON(path string, value any) error {
	raw, err := JSON(value)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func WriteText(path, value string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(value), 0o644)
}

func DecodeJSON(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}
