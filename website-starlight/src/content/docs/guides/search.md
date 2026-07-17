---
title: Search your documents
description: Get the most out of hybrid search, tags, and citations.
---

`hay search` runs both semantic and keyword retrieval and merges the results. This guide shows how to use each strength and how to narrow results when your index grows.

## Search by meaning or by exact term

Both of these work well, for different reasons:

```sh
hay search "what happens if we stop paying"     # meaning: finds payment default clauses
hay search "MSA-2024-117"                       # exact: finds that contract number
```

Semantic retrieval catches paraphrases and related concepts. Keyword retrieval (SQLite FTS5 with BM25 ranking) catches identifiers, names, and rare terms that embeddings blur. You never choose between them; every query runs both and the rankings are fused.

## Read the citations

```
 1. ~/cases/acme/contract.pdf · page 12
    ...the indemnity cap shall not exceed two million dollars...
```

PDFs cite a page. Formats without fixed pages (Markdown, text, docx) cite a chunk position instead. The citation is the contract: if a result cannot tell you where it came from, it does not belong in the output.

## Narrow with tags

Tag folders when you add them, then scope searches:

```sh
hay add ~/cases/acme --tag acme
hay add ~/personal/notes --tag personal

hay search "deposition schedule" --tag acme
```

A folder configured with `hay init` gets its tag from `.haypile.yml`, so you set it once per folder rather than remembering flags.

## Control the result count

```sh
hay search "termination" --limit 25
```

The default is 10. Results are ranked, so the first few are usually what you want.

## Freshness is automatic

While the daemon runs (it starts automatically on `hay add`), saved files are re-indexed within seconds, deleted files leave the index, and new files in watched folders appear on their own. There is no re-index command because you should never need one. To check what is indexed right now:

```sh
hay list
```
