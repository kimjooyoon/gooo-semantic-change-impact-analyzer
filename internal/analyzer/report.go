package analyzer

import (
	"fmt"
	"strings"
)

func RenderScenarioReport(graph ImpactGraph, receipt ImpactReceipt) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Semantic impact: %s\n\n", graph.CaseID)
	fmt.Fprintf(&b, "decision: `%s`\n\n", graph.Decision)
	fmt.Fprintf(&b, "| exact count | value |\n|---|---:|\n| impacted nodes | %d |\n| unaffected nodes | %d |\n| causal edges | %d |\n| minimal frontier edges | %d |\n| selected proof obligations | %d |\n", graph.Counts.ImpactedNodes, graph.Counts.UnaffectedNodes, graph.Counts.CausalEdges, graph.Counts.MinimalFrontier, graph.Counts.SelectedProofObligations)
	b.WriteString("\nchanged nodes: `" + strings.Join(graph.ChangedNodes, "`, `") + "`\n")
	b.WriteString("impacted nodes: `" + strings.Join(graph.ImpactedNodes, "`, `") + "`\n")
	b.WriteString("unaffected nodes: `" + strings.Join(graph.UnaffectedNodes, "`, `") + "`\n")
	b.WriteString("causal edges: `" + strings.Join(graph.CausalEdges, "`, `") + "`\n")
	b.WriteString("minimal frontier: `" + strings.Join(graph.MinimalFrontier, "`, `") + "`\n")
	b.WriteString("selected proof obligations: `" + strings.Join(graph.SelectedProofObligations, "`, `") + "`\n")
	if graph.Unknown != nil {
		fmt.Fprintf(&b, "\nunknown: stage=%s; step=%s; reason=%s; unknown_class=%s; next_operation=%s; blocked_by=`%s`\n", graph.Unknown.Stage, graph.Unknown.Step, graph.Unknown.Reason, graph.Unknown.UnknownClass, graph.Unknown.NextOperation, strings.Join(graph.Unknown.BlockedBy, "`, `"))
	}
	fmt.Fprintf(&b, "\nreplay runs: %d; replay match: %t\n", receipt.Replay.Runs, receipt.Replay.Match)
	return b.String()
}

func RenderConformanceReport(index ConformanceIndex) string {
	var b strings.Builder
	b.WriteString("# Gooo semantic change impact conformance\n\n")
	fmt.Fprintf(&b, "fixed scenarios: %d\n\n", index.Denominator)
	b.WriteString("| case_id | expected | decision | impacted nodes | unaffected nodes | causal edges | minimal frontier | selected proof obligations |\n|---|---|---|---:|---:|---:|---:|---:|\n")
	for _, scenario := range index.Scenarios {
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %d | %d | %d |\n", scenario.CaseID, scenario.Expected, scenario.Decision, scenario.ImpactedNodes, scenario.UnaffectedNodes, scenario.CausalEdges, scenario.MinimalFrontier, scenario.SelectedProofObligations)
	}
	fmt.Fprintf(&b, "\nexact decision counts: CLOSED=%d, UNKNOWN=%d, REFUTED=%d\n", index.Closed, index.Unknown, index.Refuted)
	fmt.Fprintf(&b, "tests: total=%d, selected=%d, executed=%d, reused=%d, failed=%d, unknown=%d\n", index.Tests.Total, index.Tests.Selected, index.Tests.Executed, index.Tests.Reused, index.Tests.Failed, index.Tests.Unknown)
	b.WriteString("\nResolution precedence: REFUTED > UNKNOWN > CLOSED.\n")
	b.WriteString("Improvement: UNKNOWN because no exact before/after integer pair with matching scenario, source, contract, toolchain, and runner was provided.\n")
	return b.String()
}
