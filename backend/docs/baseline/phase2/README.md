# Phase 2 Baseline Assets

This folder now carries the minimal Phase2 non-RRF baseline assets for L0 and L7 evaluation work.

Files:

- `baseline_snapshot.json`: current working snapshot in the repo.
- `non_rrf_baseline_snapshot.template.json`: clean template for creating a frozen non-RRF baseline snapshot.

Related templates:

- `backend/scripts/evaluation/non_rrf_dataset.template.json`
- `backend/scripts/evaluation/non_rrf_profiles.template.json`

Expected usage:

1. Copy the non-RRF dataset/profile templates and replace placeholder chunk IDs.
2. Freeze the config and strategy digest into a snapshot JSON.
3. Run `cmd/retrieval-eval` against the non-RRF dataset/profile pair.
4. Fill the L7 regression report template with both quality and route-behaviour metrics.
