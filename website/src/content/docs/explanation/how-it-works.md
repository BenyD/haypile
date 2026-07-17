---
title: How search works
description: The pipeline from a saved file to a cited answer, and why it is built this way.
---

This page explains what happens between `hay add` and a search result. Nothing here is required reading to use Haypile; it exists because understanding a tool builds the right instincts for it.

## The pipeline

```
folders -> watcher -> extract -> chunk -> embed -> SQLite
                                                     |
you <- citations <- fuse <- keyword search + vector search
```

### Extract

Each format gets a real parser. PDFs go through PDFium, the same engine Chrome uses to render them, compiled to WebAssembly so it runs inside the Go binary with no native dependencies. Extraction is per page, and the page number travels with the text from here on; it is what makes `contract.pdf · page 12` possible later. Word documents are unzipped and their XML parsed directly. Markdown is sectioned at its headings.

### Chunk

Search operates on chunks of roughly 500 tokens, not whole documents. Chunking is structure-aware: a chunk never crosses a page or heading boundary, so a citation always points at one coherent place. Consecutive chunks overlap slightly, so a sentence that straddles a boundary is findable from either side.

### Embed

Every chunk goes through a sentence-embedding model (all-MiniLM-L6-v2) that maps text to a 384-dimensional vector where similar meanings land near each other. The model ships inside the binary, quantized to 23MB, and runs in pure Go. This is unusual and deliberate: it is why semantic search works on a fresh install with zero downloads, no Python, and no GPU. Identical text is never embedded twice; a content-addressed cache sees through file renames and re-indexes.

### Store

Everything lands in one SQLite file: the chunk text in an FTS5 full-text index, the vectors as blobs beside them. SQLite in WAL mode means searches keep answering while indexing writes. There is no server, no schema to migrate by hand, and backup is `cp`.

## Why two searches

Semantic search and keyword search fail in opposite ways.

Embeddings capture meaning, so "agreement cancellation" finds a termination clause it shares no words with. But they blur exact identifiers: `MSA-2024-117` and `MSA-2024-118` embed nearly identically, and only one of them is your contract.

Keyword search (BM25 over FTS5) nails identifiers, names, and rare terms, but has no idea that "cancel" and "terminate" are the same intent.

So every query runs both, and the two rankings are merged with Reciprocal Rank Fusion: each result scores by its rank position in each list, which needs no tuning and no score normalization, since BM25 scores and cosine similarities live on unrelated scales. A result that both retrievers like rises to the top; a result only one likes still surfaces.

## Why citations are non-negotiable

Retrieval is probabilistic. The ranker can be wrong, and when `hay ask` hands passages to a small local LLM, the model can misread them. Citations are the honesty mechanism: every result and every answer points at a file and page you can open. The design rule inside the codebase is that if output cannot be traced to a source, it does not ship.

## What the eval set is for

Retrieval quality regressions are invisible: the code compiles, tests pass, and results are quietly worse. Haypile carries a query set with expected results (in [`eval/`](https://github.com/BenyD/haypile/tree/main/eval)) that runs in CI on every change that touches retrieval. Chunk sizes, fusion constants, and model choices change only when that eval says the change is an improvement. During development it has already caught a real regression that keyword-tuning introduced; that is the mechanism working as intended.
