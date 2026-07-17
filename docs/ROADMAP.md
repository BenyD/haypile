# Haypile roadmap and decisions

The living plan: what ships now, what comes next, and the decisions and
trust commitments behind them. The README is the front door; this is the
deeper "why". (It replaces the pre-launch PRD, whose scope has shipped.)

## Roadmap

### Now (v0.x)

- CLI, REST API, and MCP server, all in one binary.
- Formats: Markdown, plain text, PDF (text layer), docx, pptx, HTML.
- Bundled embedder with hybrid search (FTS5 keyword + vectors, merged with
  Reciprocal Rank Fusion), `hay ask` against any local OpenAI-compatible
  endpoint, and per-folder setup via `hay init`.
- Install: Homebrew, the shell one-liner (`curl haypile.sh | sh`), and the
  PowerShell one-liner for Windows (`irm haypile.sh/install.ps1 | iex`).

### Next (v1.x)

- OCR for scanned PDFs. Image-only pages skip today; the goal is to detect
  them, OCR the text, and cite them like any other page, without breaking
  the bundled / offline / one-binary contract.
- Continued Windows polish beyond the installer.

### Later

- **v1.5, `hay web`:** a minimal bundled local web UI served by the daemon
  on localhost. Search box, answer with clickable citations, source
  preview. Free, AGPL, single-user. The entry point for non-technical
  users, developed fully in the open. Community UIs on the API are
  encouraged in the meantime.
- **v2:** optional larger embedding models, and an ANN index for very large
  corpora (brute-force vector scan is fine into the hundreds of thousands).
- **Pro (paid):** a team layer for offices: auth, roles, audit logs, shared
  indexes, and the team web UI. The dividing line is single-user vs. team,
  never engine vs. interface.

## Trust commitments

Haypile's brand is trust, so these are versioned with the code and never
quietly redrawn:

1. **The open/paid boundary is declared upfront.** Open forever under
   AGPL-3.0: indexing, search, ask, REST API, MCP, CLI, and the coming
   single-user web UI. The full single-user product, no feature hostages.
   Paid, later: team features (auth, roles, audit logs, shared indexes) and
   a hosted version.
2. **"Zero external connections" is a verifiable feature, not a claim.**
   `hay status` reports outbound connections; the target is 0. No telemetry.
   If that ever changes it will be opt-in, documented loudly, and off by
   default.
3. **No silent network behavior.** Any future update or license check is a
   single disclosed request; the product works offline forever after. A
   local-first tool must never phone home to keep working.
4. **All development happens in this public repo.** If a component ever
   cannot be open (say, a hosted control plane), it ships as a clearly
   branded separate product, never mixed into the open binary's download.

Why AGPL over MIT: MIT would let a cloud provider ship a hosted Haypile
without contributing back. AGPL keeps the product free for every actual
user while requiring anyone who offers it as a service to open-source their
changes. It is the standard choice for open-source with a business behind it
(Grafana, Plausible, Cal.com), and it reads as a feature to the audience
that cares about staying local.

## Decisions

Resolved calls worth keeping a record of:

- **Embeddings ship inside the binary; there is no LLM dependency for
  search.** Both keyword and semantic search work out of the box, so install
  to first search stays under two minutes with zero external setup. The
  default model (all-MiniLM-L6-v2, quantized) was chosen by the eval set;
  accuracy wins, binary size breaks ties. `hay ask` delegates generation to
  whatever OpenAI-compatible local server the user already runs.
- **Chunking starts at ~500 tokens with 10 to 15% overlap, split on document
  structure.** These are knobs the eval harness tunes, not values to debate
  upfront; the eval set is the tie-breaker for every retrieval parameter.
- **MCP transport is Streamable HTTP first, stdio second.** The daemon
  already serves HTTP on :11500, so HTTP is nearly free; `hay mcp-stdio` is
  the thin process-launch fallback for clients that prefer it.
- **No telemetry, period.** "Zero external connections" and "anonymous usage
  pings" cannot share a README. Adoption is measured server-side (installer
  download counts, Homebrew analytics, GitHub stars and issues) without ever
  touching the user's machine.
- **Retrieval quality is measured, not vibed.** An eval set of expected
  results runs on every retrieval-affecting change; no silent regressions.

## Non-goals (for now)

- No auth, multi-user, or roles in the free tier (that is the paid team
  layer).
- No cloud sync or hosted service yet.
- No bundled LLM. Bundling an embedding model is megabytes; bundling an LLM
  is gigabytes. Generation stays delegated to a local endpoint.
- No connectors to SaaS sources (Slack, Notion, Gmail). Filesystem only.
