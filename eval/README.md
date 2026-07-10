# Retrieval evaluation

Accuracy is Haypile's #1 priority, and it is measured, not assumed. This
directory holds the eval set: real queries with known-correct results, run
against a fixture corpus on every retrieval-affecting change.

- `queries.yaml` — the query set. Each entry names the query and the
  document(s)/section(s) a correct retrieval must rank highly.
- `corpus/` — the fixture documents the queries run against (added at M0/M2).

Rules:

1. Every change to chunking, embedding, ranking, or extraction runs the eval
   before merge. A score drop is a failing check, not a footnote.
2. When a real-world retrieval miss is found, it becomes a new eval case
   before it gets fixed (same discipline as regression tests).
3. Scores are recorded per release so quality trends are visible over time.
