#!/usr/bin/env bash
set -euo pipefail

repo_root="${GITHUB_WORKSPACE:-$(pwd)}"
runner_temp="${RUNNER_TEMP:-$(mktemp -d)}"
run_id="${GITHUB_RUN_ID:-local}"
work="${runner_temp}/gooo-semantic-change-impact-analyzer-${run_id}"
mkdir -p "$work"
metrics="$work/stage-metrics.tsv"
: > "$metrics"

if [[ "$(go env GOVERSION)" != "go1.27.0" ]]; then
  echo "Go 1.27.0 is required" >&2
  exit 1
fi

run_stage() {
  local stage="$1"
  shift
  local started ended wall rss
  started=$(date +%s%3N)
  /usr/bin/time -f '%M' -o "$work/${stage}.rss" "$@"
  ended=$(date +%s%3N)
  wall=$((ended - started))
  rss=$(tr -d '[:space:]' < "$work/${stage}.rss")
  printf '%s\t%s\t%s\n' "$stage" "$wall" "$rss" | tee -a "$metrics"
}

metric_value() {
  awk -v wanted_stage="$1" '$1 == wanted_stage {print $2}' "$metrics"
}

metric_rss() {
  awk -v wanted_stage="$1" '$1 == wanted_stage {print $3}' "$metrics"
}

before_status=$(git -C "$repo_root" status --porcelain=v1 -z --untracked-files=all | sha256sum | awk '{print $1}')
go_files=$(git -C "$repo_root" ls-files '*.go')
test -n "$go_files"
test -z "$(cd "$repo_root" && gofmt -l $go_files)"

run_stage build go build -trimpath -o "$work/gooo-semantic-change-impact-analyzer" ./cmd/gooo-semantic-change-impact-analyzer
run_stage test go test ./...
run_stage vet go vet ./...

compile_dir="$work/compile"
mkdir -p "$compile_dir"
run_stage compile "$work/gooo-semantic-change-impact-analyzer" compile \
  --source "$repo_root/examples/semantic-change-impact-analyzer-v1/main.gooo" \
  --contract "$repo_root/contracts/impact-denominator-v1.json" \
  --output-ir "$compile_dir/semantic-ir.json" \
  --output-go "$compile_dir/semantic.gooo.go"

run_stage integration "$work/gooo-semantic-change-impact-analyzer" analyze \
  --source "$repo_root/examples/semantic-change-impact-analyzer-v1/main.gooo" \
  --contract "$repo_root/contracts/impact-denominator-v1.json" \
  --fixture "$repo_root/fixtures/cases/symbol-body-precise-dependent.json" \
  --output-dir "$work/integration"

run_stage conformance "$work/gooo-semantic-change-impact-analyzer" conformance \
  --root "$repo_root" \
  --source "$repo_root/examples/semantic-change-impact-analyzer-v1/main.gooo" \
  --contract "$repo_root/contracts/impact-denominator-v1.json" \
  --fixtures "$repo_root/fixtures/cases" \
  --output-dir "$work/conformance" \
  --runner "github-actions/ubuntu-latest" \
  --tests-total 8 --tests-selected 8 --tests-executed 8 --tests-reused 0 --tests-failed 0 --tests-unknown 0

compile_wall_ms=$(metric_value compile)
compile_peak_rss_kib=$(metric_rss compile)
build_wall_ms=$(metric_value build)
build_peak_rss_kib=$(metric_rss build)
test_wall_ms=$(metric_value test)
test_peak_rss_kib=$(metric_rss test)
conformance_wall_ms=$(metric_value conformance)
conformance_peak_rss_kib=$(metric_rss conformance)
integration_wall_ms=$(metric_value integration)
integration_peak_rss_kib=$(metric_rss integration)
jq -n \
  --argjson compile_wall_ms "$compile_wall_ms" \
  --argjson compile_peak_rss_kib "$compile_peak_rss_kib" \
  --argjson build_wall_ms "$build_wall_ms" \
  --argjson build_peak_rss_kib "$build_peak_rss_kib" \
  --argjson test_wall_ms "$test_wall_ms" \
  --argjson test_peak_rss_kib "$test_peak_rss_kib" \
  --argjson conformance_wall_ms "$conformance_wall_ms" \
  --argjson conformance_peak_rss_kib "$conformance_peak_rss_kib" \
  --argjson integration_wall_ms "$integration_wall_ms" \
  --argjson integration_peak_rss_kib "$integration_peak_rss_kib" \
  '{compile_wall_ms,compile_peak_rss_kib,build_wall_ms,build_peak_rss_kib,test_wall_ms,test_peak_rss_kib,conformance_wall_ms,conformance_peak_rss_kib,integration_wall_ms,integration_peak_rss_kib}' \
  > "$work/conformance/ci-stage-metrics.json"

jq -e '
  .schema == "gooo/semantic-change-impact-analyzer/conformance-index/v1" and
  .denominator == 8 and
  (.scenarios | length) == 8 and
  .closed == 4 and .unknown == 2 and .refuted == 2 and
  .precedence == ["REFUTED", "UNKNOWN", "CLOSED"] and
  .tests == {total:8,selected:8,executed:8,reused:0,failed:0,unknown:0} and
  .authority.repository_writes == 0 and .authority.commits == 0 and
  .authority.pushes == 0 and .authority.merges == 0 and .authority.releases == 0 and
  .authority.caller_owned_output == true and
  .structural_pair.measurement_field_coverage == 1 and
  .structural_pair.scan_fail_closed == 1 and
  .improvement.state == "UNKNOWN" and
  .improvement.unknown.stage != "" and .improvement.unknown.step != "" and
  .improvement.unknown.reason != "" and .improvement.unknown.unknown_class != "" and
  .improvement.unknown.next_operation != "" and (.improvement.unknown.blocked_by | length) > 0
' "$work/conformance/conformance-index.json"

for scenario in $(find "$work/conformance/scenarios" -mindepth 1 -maxdepth 1 -type d -print | sort); do
  jq -e '
    (.counts.impacted_nodes | type) == "number" and
    (.counts.unaffected_nodes | type) == "number" and
    (.counts.causal_edges | type) == "number" and
    (.counts.minimal_frontier | type) == "number" and
    (.counts.selected_proof_obligations | type) == "number"
  ' "$scenario/impact-graph.json"
  if [[ "$(jq -r '.decision' "$scenario/impact-graph.json")" == "UNKNOWN" ]]; then
    jq -e '.unknown.stage != "" and .unknown.step != "" and .unknown.reason != "" and .unknown.unknown_class != "" and .unknown.next_operation != "" and (.unknown.blocked_by | type) == "array"' "$scenario/impact-graph.json"
  fi
done

test "$(find "$work/conformance/scenarios" -type f -name impact-graph.json | wc -l | tr -d ' ')" = 8
test -s "$work/conformance/impact-graph.json"
test -s "$work/conformance/impact-receipt.json"
test -s "$work/conformance/human-report.md"
if ! find "$work/conformance" -type f \( -name '*.json' -o -name '*.md' \) -print0 | sort -z > "$work/scan-files.bin"; then
  echo "could not enumerate the explicit qualitative-scan file scope" >&2
  exit 1
fi
scan_files=()
while IFS= read -r -d '' file; do
  scan_files+=("$file")
done < "$work/scan-files.bin"
test "${#scan_files[@]}" -gt 0
if grep -E -i -n 'percentage|percent|score|estimated' "${scan_files[@]}"; then
  echo "qualitative score or percentage was emitted" >&2
  exit 1
else
  scan_status=$?
  if [[ "$scan_status" -ne 1 ]]; then
    echo "qualitative scan could not read its explicit file scope" >&2
    exit "$scan_status"
  fi
fi
scan_fail_closed=1
test "$(grep -c '^activity ' "$repo_root/examples/semantic-change-impact-analyzer-v1/main.gooo")" = 6
test "$(grep -E -c 'activity=.*(ParseSemanticDelta|BindTypedDependencies|ComputeCausalFrontier|ClassifyImpact|EmitImpactReceipt|VerifyImpactReplay)' "$repo_root/examples/semantic-change-impact-analyzer-v1/main.gooo")" = 6

jq -e '.schema == "gooo/semantic-change-impact-analyzer/semantic-ir/v1" and (.activities | length) == 6 and ([.activities[].name] | length) == 6' "$compile_dir/semantic-ir.json"
test "$(grep -E -c 'ParseSemanticDelta|BindTypedDependencies|ComputeCausalFrontier|ClassifyImpact|EmitImpactReceipt|VerifyImpactReplay' "$compile_dir/semantic.gooo.go")" = 1

jq -e '
  ([.compile_wall_ms,.compile_peak_rss_kib,.build_wall_ms,.build_peak_rss_kib,
    .test_wall_ms,.test_peak_rss_kib,.conformance_wall_ms,.conformance_peak_rss_kib,
    .integration_wall_ms,.integration_peak_rss_kib] | all(type == "number" and floor == .))
' "$work/conformance/ci-stage-metrics.json"
test "$scan_fail_closed" = 1

after_status=$(git -C "$repo_root" status --porcelain=v1 -z --untracked-files=all | sha256sum | awk '{print $1}')
test "$before_status" = "$after_status"

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  cat "$work/conformance/ci-summary.md" >> "$GITHUB_STEP_SUMMARY"
  {
    echo
    echo "## CI stage metrics"
    echo
    echo '| stage | wall_ms | peak_rss_kib |'
    echo '|---|---:|---:|'
    awk -F '\t' '{printf "| %s | %s | %s |\n", $1, $2, $3}' "$metrics" >> "$GITHUB_STEP_SUMMARY"
  } >> "$GITHUB_STEP_SUMMARY"
fi
