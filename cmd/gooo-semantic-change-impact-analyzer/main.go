package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kimjooyoon/gooo-semantic-change-impact-analyzer/internal/analyzer"
)

func main() {
	if len(os.Args) < 2 {
		fatal("command is required: compile, analyze, or conformance")
	}
	switch os.Args[1] {
	case "compile":
		compile(os.Args[2:])
	case "analyze":
		analyze(os.Args[2:])
	case "conformance":
		conformance(os.Args[2:])
	default:
		fatal("unknown command %q", os.Args[1])
	}
}

func compile(args []string) {
	set := flag.NewFlagSet("compile", flag.ExitOnError)
	source := set.String("source", "", "released .gooo source")
	contract := set.String("contract", "", "fixed denominator contract")
	outputIR := set.String("output-ir", "", "absolute caller-owned semantic IR output")
	outputGo := set.String("output-go", "", "absolute caller-owned generated binding output")
	set.Parse(args)
	if *source == "" || *contract == "" || *outputIR == "" || *outputGo == "" {
		fatal("compile requires --source, --contract, --output-ir, and --output-go")
	}
	if err := analyzer.Compile(*source, *contract, *outputIR, *outputGo); err != nil {
		fatal(err.Error())
	}
	fmt.Println("compiled released semantic graph binding")
}

func analyze(args []string) {
	set := flag.NewFlagSet("analyze", flag.ExitOnError)
	sourcePath := set.String("source", "", "released .gooo source")
	contractPath := set.String("contract", "", "fixed denominator contract")
	fixturePath := set.String("fixture", "", "one fixed scenario JSON fixture")
	outputPath := set.String("output-dir", "", "absolute caller-owned output directory")
	tests := testFlags(set)
	set.Parse(args)
	if *sourcePath == "" || *contractPath == "" || *fixturePath == "" || *outputPath == "" {
		fatal("analyze requires --source, --contract, --fixture, and --output-dir")
	}
	if err := analyzer.EnsureCallerDirectory(*outputPath); err != nil {
		fatal(err.Error())
	}
	if err := os.MkdirAll(*outputPath, 0o755); err != nil {
		fatal(err.Error())
	}
	sourceRaw, err := os.ReadFile(*sourcePath)
	if err != nil {
		fatal(err.Error())
	}
	contract, contractRaw, err := analyzer.LoadContract(*contractPath)
	if err != nil {
		fatal(err.Error())
	}
	_ = contract
	fixture, err := analyzer.LoadFixture(*fixturePath)
	if err != nil {
		fatal(err.Error())
	}
	result, err := analyzer.AnalyzeFixture(fixture, sourceRaw, contractRaw, runtimeOptions(tests))
	if err != nil {
		fatal(err.Error())
	}
	if err := analyzer.WriteJSON(filepath.Join(*outputPath, "impact-graph.json"), result.Graph); err != nil {
		fatal(err.Error())
	}
	if err := analyzer.WriteJSON(filepath.Join(*outputPath, "impact-receipt.json"), result.Receipt); err != nil {
		fatal(err.Error())
	}
	if err := analyzer.WriteText(filepath.Join(*outputPath, "human-report.md"), result.Report); err != nil {
		fatal(err.Error())
	}
	printSummary(result.Receipt.Decision, result.Graph.Counts)
}

func conformance(args []string) {
	set := flag.NewFlagSet("conformance", flag.ExitOnError)
	root := set.String("root", ".", "repository root for inventory")
	sourcePath := set.String("source", "", "released .gooo source")
	contractPath := set.String("contract", "", "fixed denominator contract")
	fixturesPath := set.String("fixtures", "", "fixed scenario fixture directory")
	outputPath := set.String("output-dir", "", "absolute caller-owned output directory")
	runner := set.String("runner", "github-actions/ubuntu-latest", "runner identity for evidence")
	tests := testFlags(set)
	set.Parse(args)
	if *sourcePath == "" || *contractPath == "" || *fixturesPath == "" || *outputPath == "" {
		fatal("conformance requires --source, --contract, --fixtures, and --output-dir")
	}
	index, err := analyzer.RunConformance(*root, *sourcePath, *contractPath, *fixturesPath, *outputPath, analyzer.RuntimeOptions{Tests: runtimeOptions(tests).Tests, Runner: *runner})
	if err != nil {
		fatal(err.Error())
	}
	printIndex(index)
}

type testFlagValues struct {
	total, selected, executed, reused, failed, unknown *int
}

func testFlags(set *flag.FlagSet) testFlagValues {
	return testFlagValues{
		total:    set.Int("tests-total", 8, "exact CI test total"),
		selected: set.Int("tests-selected", 8, "exact CI selected test count"),
		executed: set.Int("tests-executed", 8, "exact CI executed test count"),
		reused:   set.Int("tests-reused", 0, "exact reused test count"),
		failed:   set.Int("tests-failed", 0, "exact failed test count"),
		unknown:  set.Int("tests-unknown", 0, "exact unknown test count"),
	}
}

func runtimeOptions(values testFlagValues) analyzer.RuntimeOptions {
	return analyzer.RuntimeOptions{Tests: analyzer.TestMetrics{Total: *values.total, Selected: *values.selected, Executed: *values.executed, Reused: *values.reused, Failed: *values.failed, Unknown: *values.unknown}}
}

func printSummary(decision string, counts analyzer.Counts) {
	value := struct {
		Decision string          `json:"decision"`
		Counts   analyzer.Counts `json:"counts"`
	}{Decision: decision, Counts: counts}
	raw, err := json.Marshal(value)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Println(string(raw))
}

func printIndex(index analyzer.ConformanceIndex) {
	value := struct {
		Decision    string `json:"decision"`
		Denominator int    `json:"denominator"`
		Closed      int    `json:"closed"`
		Unknown     int    `json:"unknown"`
		Refuted     int    `json:"refuted"`
	}{Decision: index.Decision, Denominator: index.Denominator, Closed: index.Closed, Unknown: index.Unknown, Refuted: index.Refuted}
	raw, err := json.Marshal(value)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Println(string(raw))
}

func fatal(format string, values ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", values...)
	os.Exit(1)
}
