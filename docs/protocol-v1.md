# Semantic change impact protocol v1

The analyzer consumes two released semantic graphs. A graph node has a stable
identifier, a semantic kind, and a semantic digest. Every dependency edge has
stable endpoints, a non-empty type, and the source and target digests it was
bound against.

`ParseSemanticDelta` identifies changed semantic nodes and graph-level
contradictions. `BindTypedDependencies` verifies endpoint presence, edge type,
and digest freshness. `ComputeCausalFrontier` walks only valid after-graph
edges from changed nodes. `ClassifyImpact` resolves contradictions before
uncertainty and uncertainty before closure. `EmitImpactReceipt` produces the
machine receipts; `VerifyImpactReplay` reruns the same computation and compares
the exact serialized result.

The minimal frontier is the first valid edge leaving each changed node on a
reachable path. A proof obligation is selected exactly when its proof node is
in the impacted descendant set. The proof graph is therefore complete before
any downstream test selection could occur.

An absent target mapping is `UNKNOWN / DIRECT_MISSING`. An edge whose bound
digest no longer matches its endpoint is `UNKNOWN / STALE`. Omitting a
declared known dependent or asserting no impact while the semantic graph
digest changed is a contradiction and is `REFUTED`.

The emitted stage measurements are integer `wall_ms` and `peak_rss_kib`
observations. Test fields are integer observations with no reuse claim unless
the caller supplies an exact reusable receipt.
