---
name: haypile
description: Search the user's local documents (PDF, docx, pptx, markdown, text, HTML) through Haypile, a local search daemon. Use when the user asks what their files say, wants passages from contracts, papers, or notes, or wants a folder indexed for search. Every result carries a file and page citation. Runs entirely on localhost.
---

# Haypile: local document search for agents

Haypile is a single binary (`hay`) that indexes the user's folders into a local SQLite database and serves hybrid (semantic plus keyword) search on `localhost:11500`. The embedding model ships inside the binary. Outbound connections: 0.

## Check it is available

```sh
hay status
```

The daemon autostarts on demand and listens only on localhost. If `hay` is not installed:

```sh
brew install BenyD/tap/hay
```

Windows (PowerShell): `irm https://haypile.sh/install.ps1 | iex`

## Search, preferred path: MCP tools

If this client is connected to the Haypile MCP server, use its tools directly:

- `search_documents`: hybrid search. Arguments: `query` (required), `tag` (optional, restricts to one indexed folder's tag), `limit` (optional). Returns ranked passages, each with its source file and page.
- `list_sources`: the indexed folders and their document counts.

Not connected yet? Add it:

```sh
claude mcp add --transport http haypile http://localhost:11500/mcp
```

Clients that launch a process instead of speaking HTTP can use the stdio bridge: command `hay`, args `["mcp-stdio"]`.

## Search, fallback paths: CLI and REST

```sh
hay search "termination clause"
```

```sh
curl -X POST localhost:11500/api/query -d '{"query": "termination clause"}'
```

## Index new content

```sh
hay add ~/path/to/folder     # index a folder (or single file) and watch it for changes
hay list                     # what is indexed
hay remove ~/path/to/folder  # un-index a folder
```

Indexing is local and incremental. A saved file is searchable within seconds. For a folder the user works in regularly, `hay init` writes a per-folder config (tag, exclude patterns) and can wire up `.mcp.json`; `hay init --yes` runs unattended.

## Rules for answering from results

- Always cite. Carry each passage's file path and page into the answer.
- When exact wording matters (contracts, legal text, quotes), quote the passage rather than paraphrasing it.
- If search returns nothing, say so. Do not fill the gap from general knowledge. Offer to index the folder that would contain the answer.
- Scanned PDFs enter the index only when a local vision model handles OCR (`hay llm setup` sets one up). If an expected document is missing from results, this is a likely cause; `hay add` reports skipped scans.
