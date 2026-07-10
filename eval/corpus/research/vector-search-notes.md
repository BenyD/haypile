# Notes: vector search and hybrid retrieval

Reading survey papers on retrieval for RAG systems.

## Brute force vs ANN

Exact (brute-force) nearest-neighbor scan is O(n) per query but with SIMD it
stays under 100ms up to roughly a million vectors — fine for personal-scale
corpora. Approximate indexes like HNSW (hierarchical navigable small world
graphs) trade a little recall for large speedups; they matter at
tens-of-millions scale, at the cost of memory overhead and slower inserts.

## Hybrid retrieval

Dense (vector) retrieval misses exact identifiers; sparse (keyword/BM25)
retrieval misses paraphrases. Combining both consistently beats either alone.

Reciprocal rank fusion (RRF) is the boring, robust way to merge ranked lists:
score each document by the sum of 1/(k + rank) across lists, k around 60. No
score normalization needed, works even when the two retrievers' scores are on
completely different scales.

## Chunking

Retrieval quality is more sensitive to chunking than to embedding model
choice in several ablations. Structure-aware splitting (headings, then
paragraphs) beats fixed-size windows; a contextual prefix with the document
title measurably helps disambiguation.
