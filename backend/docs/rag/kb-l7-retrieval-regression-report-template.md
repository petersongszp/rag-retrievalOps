# L7 Retrieval Regression Report Template

## 1. Change Summary

- Change summary:
- Scope:
- Release target:
- Dataset version:
- Profile version:

## 2. Baseline vs Candidate

- Baseline:
- Candidate:
- Recall@K delta:
- MRR delta:
- nDCG delta:
- Citation accuracy delta:
- Dense hit rate delta:
- Sparse hit rate delta:
- Dense participation rate delta:
- Sparse participation rate delta:
- Primary dense rate delta:
- Primary sparse rate delta:
- Empty rate delta:
- P95 latency delta:

## 3. Strategy Metrics

| Strategy | Recall@K | MRR | nDCG | Citation Acc | Dense Hit | Sparse Hit | Dense Part. | Sparse Part. | Primary Dense | Primary Sparse | Empty Rate | P50(ms) | P95(ms) |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| baseline |  |  |  |  |  |  |  |  |  |  |  |  |  |
| candidate |  |  |  |  |  |  |  |  |  |  |  |  |  |

## 4. Route Analysis

- Dense hits / sparse hits:
- Dense participation / sparse participation:
- Primary route distribution:
- Empty-result cases:
- Cases where sparse hit but did not become primary:

## 5. Findings

- Quality risks:
- Latency risks:
- Dataset coverage gaps:
- Recommend rollout:

## 6. Rollback Plan

1. Revert to the frozen non-RRF baseline profile.
2. Reset dense/sparse weights to the baseline values.
3. Disable the candidate-only tuning knobs if regression persists.
4. Attach the failed regression report to the rollback ticket.
