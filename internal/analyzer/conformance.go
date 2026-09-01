package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func RunConformance(root, sourcePath, contractPath, fixturesPath, outputPath string, options RuntimeOptions) (ConformanceIndex, error) {
	if err := EnsureCallerDirectory(outputPath); err != nil {
		return ConformanceIndex{}, err
	}
	if err := os.MkdirAll(outputPath, 0o755); err != nil {
		return ConformanceIndex{}, err
	}
	sourceRaw, err := os.ReadFile(sourcePath)
	if err != nil {
		return ConformanceIndex{}, err
	}
	contract, contractRaw, err := LoadContract(contractPath)
	if err != nil {
		return ConformanceIndex{}, err
	}
	source, err := ParseSource(sourceRaw)
	if err != nil {
		return ConformanceIndex{}, err
	}
	if source.Release == "" {
		return ConformanceIndex{}, fmt.Errorf("released semantic graph binding is missing")
	}
	files, err := filepath.Glob(filepath.Join(fixturesPath, "*.json"))
	if err != nil {
		return ConformanceIndex{}, err
	}
	sort.Strings(files)
	if len(files) != contract.Total {
		return ConformanceIndex{}, fmt.Errorf("fixed denominator requires %d fixtures, got %d", contract.Total, len(files))
	}
	if options.Runner == "" {
		options.Runner = "github-actions/ubuntu-latest"
	}
	if err := validateTestMetrics(options.Tests); err != nil {
		return ConformanceIndex{}, err
	}
	if options.Inventory == (Inventory{}) {
		options.Inventory, err = InventoryForRoot(root)
		if err != nil {
			return ConformanceIndex{}, err
		}
	}
	start := measureStage("Conformance", func() {})
	index := ConformanceIndex{
		Schema: IndexSchema, Decision: "IMPACT_CONFORMANCE_REPORTED", Denominator: contract.Total,
		Scenarios: []ScenarioSummary{}, Precedence: append([]string(nil), Precedence...),
		Tests: options.Tests, StageMetrics: []StageMetric{start}, Authority: Authority{CallerOwnedOutput: true},
		Improvement:    unknownImprovement(DigestBytes(sourceRaw), DigestBytes(contractRaw), DigestBytes([]byte(ToolchainVersion)), DigestBytes([]byte(options.Runner))),
		StructuralPair: StructuralPair{MeasurementFieldCoverage: 1, ScanFailClosed: 1},
		Inventory:      options.Inventory,
	}
	for _, path := range files {
		fixture, err := LoadFixture(path)
		if err != nil {
			return ConformanceIndex{}, err
		}
		result, err := AnalyzeFixture(fixture, sourceRaw, contractRaw, options)
		if err != nil {
			return ConformanceIndex{}, err
		}
		ordinal, expected, err := contractScenario(contract, fixture.CaseID)
		if err != nil {
			return ConformanceIndex{}, err
		}
		summary := ScenarioSummary{
			Ordinal: ordinal, CaseID: fixture.CaseID, Expected: expected, Decision: result.Receipt.Decision,
			ImpactedNodes: result.Graph.Counts.ImpactedNodes, UnaffectedNodes: result.Graph.Counts.UnaffectedNodes,
			CausalEdges: result.Graph.Counts.CausalEdges, MinimalFrontier: result.Graph.Counts.MinimalFrontier,
			SelectedProofObligations: result.Graph.Counts.SelectedProofObligations,
			Report:                   filepath.ToSlash(filepath.Join("scenarios", fixture.CaseID, "human-report.md")),
		}
		index.Scenarios = append(index.Scenarios, summary)
		switch result.Receipt.Decision {
		case DecisionClosed:
			index.Closed++
		case DecisionUnknown:
			index.Unknown++
		case DecisionRefuted:
			index.Refuted++
		default:
			return ConformanceIndex{}, fmt.Errorf("unsupported decision %s", result.Receipt.Decision)
		}
		caseDir := filepath.Join(outputPath, "scenarios", fixture.CaseID)
		if err := WriteJSON(filepath.Join(caseDir, "impact-graph.json"), result.Graph); err != nil {
			return ConformanceIndex{}, err
		}
		if err := WriteJSON(filepath.Join(caseDir, "impact-receipt.json"), result.Receipt); err != nil {
			return ConformanceIndex{}, err
		}
		if err := WriteText(filepath.Join(caseDir, "human-report.md"), result.Report); err != nil {
			return ConformanceIndex{}, err
		}
	}
	if len(index.Scenarios) != contract.Total {
		return ConformanceIndex{}, fmt.Errorf("conformance did not evaluate the fixed denominator")
	}
	graphIndex := struct {
		Schema      string            `json:"schema"`
		Denominator int               `json:"denominator"`
		Scenarios   []ScenarioSummary `json:"scenarios"`
	}{Schema: "gooo/semantic-change-impact-analyzer/impact-graph-index/v1", Denominator: index.Denominator, Scenarios: index.Scenarios}
	if err := WriteJSON(filepath.Join(outputPath, "impact-graph.json"), graphIndex); err != nil {
		return ConformanceIndex{}, err
	}
	if err := WriteJSON(filepath.Join(outputPath, "impact-receipt.json"), index); err != nil {
		return ConformanceIndex{}, err
	}
	if err := WriteJSON(filepath.Join(outputPath, "conformance-index.json"), index); err != nil {
		return ConformanceIndex{}, err
	}
	report := RenderConformanceReport(index)
	if err := WriteText(filepath.Join(outputPath, "human-report.md"), report); err != nil {
		return ConformanceIndex{}, err
	}
	if err := WriteText(filepath.Join(outputPath, "ci-summary.md"), report); err != nil {
		return ConformanceIndex{}, err
	}
	return index, nil
}

func contractScenario(contract Contract, caseID string) (int, string, error) {
	for _, scenario := range contract.Scenarios {
		if scenario.ID == caseID {
			return scenario.Ordinal, scenario.Expected, nil
		}
	}
	return 0, "", fmt.Errorf("fixture %s is not in fixed contract", caseID)
}

func sortedScenarioIDs(scenarios []ScenarioSummary) []string {
	ids := make([]string, 0, len(scenarios))
	for _, scenario := range scenarios {
		ids = append(ids, scenario.CaseID)
	}
	sort.Strings(ids)
	return ids
}

func formatScenarioIDs(scenarios []ScenarioSummary) string {
	return strings.Join(sortedScenarioIDs(scenarios), ",")
}
