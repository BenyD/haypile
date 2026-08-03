# Real-world retrieval baseline

The fixture eval in [eval/](../) is the merge gate. This directory measures
retrieval against real public PDFs: two-column papers, long structured law,
table-heavy government publications, plain prose. The output is one number,
recall@3, recorded per release below. The number going down is a regression
even while the gate stays green.

```sh
./eval/realworld/fetch-corpus.sh     # ~47 PDFs, stays out of git
go test ./eval/realworld -v -timeout 60m
```

The test skips without the corpus or the embedding model
(`hack/fetch-model.sh`), so CI is unaffected unless both are present.

Rules:

1. Record recall@3 here for every release and for every change to chunking,
   embedding, extraction, or ranking.
2. A real-world retrieval miss found in the wild becomes a query here (or in
   the gate) before it gets fixed.
3. Queries use different words than the document on purpose; do not "fix" a
   miss by rewording the query toward the document.

## Baseline

| Date | Commit | recall@3 | Notes |
|---|---|---|---|
| 2026-08-03 | 53c740f | 0.95 (19/20) | 47 files, 9,532 chunks, bundled all-MiniLM-L6-v2. Single miss: the zero-trust paraphrase ("never trust a network location...") retrieves QUIC/HTTP RFCs over NIST 800-207; a known semantic gap, kept as-is to catch a future model that closes it. |
