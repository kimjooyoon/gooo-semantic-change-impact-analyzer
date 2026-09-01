# gooo-semantic-change-impact-analyzer

`gooo-semantic-change-impact-analyzer` computes the smallest causal frontier
between two released Gooo semantic graphs. It explains which generated
artifacts, proof obligations, and tests are downstream of a semantic change;
it does not select or execute tests.

The authority chain is:

```text
.gooo activities → released semantic graph → typed dependency edges
  → semantic delta → causal frontier → impact receipt
```

The `.gooo` declaration owns the six semantic activities. Go is only the
evaluator, generator, and runtime for that declaration. The evaluator never
uses source text or generated Go text as a semantic proxy: it binds node
digests and typed edge endpoints first, then computes descendants.

## Fixed conformance

The v1 denominator is exactly eight scenarios:

1. a symbol-body change with a precise dependent;
2. a type-contract change with a transitive dependent;
3. a comment-only change with zero semantic impact;
4. deterministic replay;
5. a missing artifact mapping (`UNKNOWN / DIRECT_MISSING`);
6. a stale dependency edge (`UNKNOWN / STALE`);
7. an omitted known dependent (`REFUTED`); and
8. an unchanged claim despite a changed semantic digest (`REFUTED`).

Every scenario records exact integer counts for impacted nodes, unaffected
nodes, causal edges, minimal-frontier edges, and selected proof obligations.
Resolution precedence is `REFUTED > UNKNOWN > CLOSED`. An UNKNOWN always
preserves `stage`, `step`, `reason`, `unknown_class`, `next_operation`, and
`blocked_by`.

No score, percentage, or estimated metric is emitted. Improvement remains
`UNKNOWN` until an exact integer before/after pair has the same scenario,
source, contract, toolchain, and runner identity.

## Caller-owned output

The CLI writes `impact-graph.json`, `impact-receipt.json`, and a human report
only below the absolute output directory supplied by the caller. The directory
must be outside this repository and empty (or absent). The repository authority
receipt is fixed at zero repository writes, commits, pushes, merges, and
releases; release automation is outside the product evaluator's authority.

```sh
go run ./cmd/gooo-semantic-change-impact-analyzer conformance \
  --source examples/semantic-change-impact-analyzer-v1/main.gooo \
  --contract contracts/impact-denominator-v1.json \
  --fixtures fixtures/cases \
  --output-dir /tmp/gooo-semantic-change-impact-analyzer
```

GitHub Actions runs the authoritative Go 1.27 formatting, build, test, vet,
integration, and conformance checks. Inventory counts exclude this root
`README.md`, `.git`, caller-owned temporary output, cache, vendor, and
toolchain internals. CI also writes `ci-stage-metrics.json` with integer
`wall_ms` and `peak_rss_kib` pairs for compile, build, test, conformance, and
integration. Its qualitative-output scan enumerates JSON/Markdown files
explicitly and fails on every scan error; `structural_pair` records the exact
binary checks `measurement_field_coverage=1` and `scan_fail_closed=1`.
