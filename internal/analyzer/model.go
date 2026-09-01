package analyzer

import "encoding/json"

const (
	GraphSchema       = "gooo/semantic-change-impact-analyzer/semantic-graph/v1"
	IRSchema          = "gooo/semantic-change-impact-analyzer/semantic-ir/v1"
	ImpactGraphSchema = "gooo/semantic-change-impact-analyzer/impact-graph/v1"
	ReceiptSchema     = "gooo/semantic-change-impact-analyzer/impact-receipt/v1"
	IndexSchema       = "gooo/semantic-change-impact-analyzer/conformance-index/v1"
	ContractSchema    = "gooo/semantic-change-impact-analyzer/denominator/v1"
	DecisionClosed    = "CLOSED"
	DecisionUnknown   = "UNKNOWN"
	DecisionRefuted   = "REFUTED"
	UnknownDirect     = "DIRECT_MISSING"
	UnknownStale      = "STALE"
	ToolchainVersion  = "go1.27.0"
)

var Precedence = []string{DecisionRefuted, DecisionUnknown, DecisionClosed}

var RequiredActivities = []string{
	"ParseSemanticDelta",
	"BindTypedDependencies",
	"ComputeCausalFrontier",
	"ClassifyImpact",
	"EmitImpactReceipt",
	"VerifyImpactReplay",
}

type Activity struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Proof     string `json:"proof"`
	Artifact  string `json:"artifact"`
	Authority string `json:"authority"`
}

type SourceDeclaration struct {
	GraphID    string
	Release    string
	Activities []Activity
}

type SemanticNode struct {
	ID             string `json:"id"`
	Kind           string `json:"kind"`
	SemanticDigest string `json:"semantic_digest"`
}

type TypedDependencyEdge struct {
	EdgeID     string `json:"edge_id"`
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	ValueType  string `json:"value_type"`
	FromDigest string `json:"from_digest"`
	ToDigest   string `json:"to_digest"`
}

type SemanticGraph struct {
	Schema         string                `json:"schema"`
	GraphID        string                `json:"graph_id"`
	ReleaseTag     string                `json:"release_tag"`
	ReleaseDigest  string                `json:"release_digest"`
	SemanticDigest string                `json:"semantic_digest"`
	Nodes          []SemanticNode        `json:"nodes"`
	Edges          []TypedDependencyEdge `json:"edges"`
}

type ContractScenario struct {
	Ordinal      int    `json:"ordinal"`
	ID           string `json:"id"`
	Expected     string `json:"expected_decision"`
	UnknownClass string `json:"unknown_class,omitempty"`
}

type Contract struct {
	Schema        string             `json:"schema"`
	DenominatorID string             `json:"denominator_id"`
	CandidateID   string             `json:"candidate_id"`
	Total         int                `json:"total"`
	Activities    []string           `json:"activities"`
	Scenarios     []ContractScenario `json:"scenarios"`
}

type ReplaySpec struct {
	Enabled bool `json:"enabled"`
	Runs    int  `json:"runs"`
}

type Claim struct {
	Decision        string `json:"decision"`
	ImpactedNodes   int    `json:"impacted_nodes"`
	CausalEdges     int    `json:"causal_edges"`
	MinimalFrontier int    `json:"minimal_frontier"`
}

type ExpectedCounts struct {
	Decision                 string `json:"decision"`
	ImpactedNodes            int    `json:"impacted_nodes"`
	UnaffectedNodes          int    `json:"unaffected_nodes"`
	CausalEdges              int    `json:"causal_edges"`
	MinimalFrontier          int    `json:"minimal_frontier"`
	SelectedProofObligations int    `json:"selected_proof_obligations"`
	UnknownClass             string `json:"unknown_class,omitempty"`
}

type CaseFixture struct {
	Schema          string         `json:"schema"`
	CaseID          string         `json:"case_id"`
	Description     string         `json:"description"`
	Before          SemanticGraph  `json:"before"`
	After           SemanticGraph  `json:"after"`
	KnownDependents []string       `json:"known_dependents"`
	Claim           Claim          `json:"claim"`
	Replay          ReplaySpec     `json:"replay"`
	Expected        ExpectedCounts `json:"expected"`
}

type Unknown struct {
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

func (u Unknown) Valid() bool {
	return u.Stage != "" && u.Step != "" && u.Reason != "" &&
		u.UnknownClass != "" && u.NextOperation != "" && u.BlockedBy != nil
}

type Counts struct {
	ImpactedNodes            int `json:"impacted_nodes"`
	UnaffectedNodes          int `json:"unaffected_nodes"`
	CausalEdges              int `json:"causal_edges"`
	MinimalFrontier          int `json:"minimal_frontier"`
	SelectedProofObligations int `json:"selected_proof_obligations"`
}

type StageMetric struct {
	Stage      string `json:"stage"`
	WallMS     int    `json:"wall_ms"`
	PeakRSSKiB int    `json:"peak_rss_kib"`
}

type TestMetrics struct {
	Total    int `json:"total"`
	Selected int `json:"selected"`
	Executed int `json:"executed"`
	Reused   int `json:"reused"`
	Failed   int `json:"failed"`
	Unknown  int `json:"unknown"`
}

type Authority struct {
	RepositoryWrites  int  `json:"repository_writes"`
	Commits           int  `json:"commits"`
	Pushes            int  `json:"pushes"`
	Merges            int  `json:"merges"`
	Releases          int  `json:"releases"`
	CallerOwnedOutput bool `json:"caller_owned_output"`
}

type StructuralPair struct {
	MeasurementFieldCoverage int `json:"measurement_field_coverage"`
	ScanFailClosed           int `json:"scan_fail_closed"`
}

type Improvement struct {
	State          string  `json:"state"`
	Reason         string  `json:"reason"`
	Scenario       string  `json:"scenario"`
	SourceDigest   string  `json:"source_digest"`
	ContractDigest string  `json:"contract_digest"`
	Toolchain      string  `json:"toolchain"`
	Runner         string  `json:"runner"`
	Before         *int    `json:"before,omitempty"`
	After          *int    `json:"after,omitempty"`
	Unknown        Unknown `json:"unknown"`
}

type Inventory struct {
	DescendantDirs     int  `json:"descendant_dirs"`
	DescendantFiles    int  `json:"descendant_files"`
	GoFiles            int  `json:"go_files"`
	GoPhysicalLines    int  `json:"go_physical_lines"`
	GoooFiles          int  `json:"gooo_files"`
	GoooPhysicalLines  int  `json:"gooo_physical_lines"`
	RootREADMEExcluded bool `json:"root_readme_excluded"`
}

type ImpactGraph struct {
	Schema                   string   `json:"schema"`
	CaseID                   string   `json:"case_id"`
	Decision                 string   `json:"decision"`
	BeforeReleaseTag         string   `json:"before_release_tag"`
	AfterReleaseTag          string   `json:"after_release_tag"`
	BeforeGraphDigest        string   `json:"before_graph_digest"`
	AfterGraphDigest         string   `json:"after_graph_digest"`
	ChangedNodes             []string `json:"changed_nodes"`
	ImpactedNodes            []string `json:"impacted_nodes"`
	UnaffectedNodes          []string `json:"unaffected_nodes"`
	CausalEdges              []string `json:"causal_edges"`
	MinimalFrontier          []string `json:"minimal_frontier"`
	SelectedProofObligations []string `json:"selected_proof_obligations"`
	Counts                   Counts   `json:"counts"`
	Unknown                  *Unknown `json:"unknown,omitempty"`
}

type ReplayObservation struct {
	Runs        int    `json:"runs"`
	Match       bool   `json:"match"`
	Fingerprint string `json:"fingerprint"`
}

type ImpactReceipt struct {
	Schema          string            `json:"schema"`
	CaseID          string            `json:"case_id"`
	Decision        string            `json:"decision"`
	Reason          string            `json:"reason"`
	Precedence      []string          `json:"precedence"`
	Counts          Counts            `json:"counts"`
	StageMetrics    []StageMetric     `json:"stage_metrics"`
	Tests           TestMetrics       `json:"tests"`
	Unknown         *Unknown          `json:"unknown,omitempty"`
	Replay          ReplayObservation `json:"replay"`
	SourceDigest    string            `json:"source_digest"`
	ContractDigest  string            `json:"contract_digest"`
	ToolchainDigest string            `json:"toolchain_digest"`
	RunnerDigest    string            `json:"runner_digest"`
	Authority       Authority         `json:"authority"`
	Improvement     Improvement       `json:"improvement"`
	StructuralPair  StructuralPair    `json:"structural_pair"`
	Inventory       Inventory         `json:"inventory"`
}

type ScenarioSummary struct {
	Ordinal                  int    `json:"ordinal"`
	CaseID                   string `json:"case_id"`
	Expected                 string `json:"expected"`
	Decision                 string `json:"decision"`
	ImpactedNodes            int    `json:"impacted_nodes"`
	UnaffectedNodes          int    `json:"unaffected_nodes"`
	CausalEdges              int    `json:"causal_edges"`
	MinimalFrontier          int    `json:"minimal_frontier"`
	SelectedProofObligations int    `json:"selected_proof_obligations"`
	Report                   string `json:"report"`
}

type ConformanceIndex struct {
	Schema         string            `json:"schema"`
	Decision       string            `json:"decision"`
	Denominator    int               `json:"denominator"`
	Scenarios      []ScenarioSummary `json:"scenarios"`
	Closed         int               `json:"closed"`
	Unknown        int               `json:"unknown"`
	Refuted        int               `json:"refuted"`
	Precedence     []string          `json:"precedence"`
	Tests          TestMetrics       `json:"tests"`
	StageMetrics   []StageMetric     `json:"stage_metrics"`
	Authority      Authority         `json:"authority"`
	Improvement    Improvement       `json:"improvement"`
	StructuralPair StructuralPair    `json:"structural_pair"`
	Inventory      Inventory         `json:"inventory"`
}

func JSON(value any) ([]byte, error) {
	return json.MarshalIndent(value, "", "  ")
}
