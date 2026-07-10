# Haypile — Product Requirements Document

**Version:** 1.0 · **Date:** July 2026 · **Owner:** Beny Dishon · **Status:** Approved for development

---

## 1. One-liner

**Haypile is Private search and Q&A for your documents.** A single Go binary that watches your folders, indexes every document, and answers questions about them — fully local, fully private, one command to install.

> Everyone says finding information in your files is like finding a needle in a haystack. Haypile is the haystack that finds its own needles.

The binary is `hay`:

```
hay add ~/Documents
hay ask "what did the Meridian contract say about termination?"
```

---

## 2. Problem

People and organizations have years of documents — contracts, case files, research papers, notes, internal docs — that are effectively unsearchable beyond filename and exact-keyword matching. Modern semantic search and RAG solve this, but every existing option forces a bad trade:

1. **Cloud AI tools** (ChatGPT, Glean, Notion AI) require uploading documents to third parties. For law firms, healthcare, finance, and privacy-conscious individuals this is unacceptable or illegal (HIPAA, GDPR, client confidentiality).
2. **Self-hosted RAG stacks** require Python environments, Docker, a separate vector database, and configuration that breaks between machines. Great for experimentation, painful for daily use.
3. **Existing local tools** are narrow: Node-based and scoped to AI coding assistants (mcp-local-rag), codebase-only (AmanMCP), heavyweight Docker deployments (RAG-Enterprise), or Python/Streamlit chat apps with manual session-based ingestion and no API (jonfairbanks/local-rag — 749 stars proves the demand; its Pipenv/Docker setup is the pain).

Nobody has shipped the `ollama pull` equivalent for documents in general: one binary, point it at folders, done.

## 3. Vision & positioning

- **Category:** local-first retrieval infrastructure. Haypile aims to be the thing developers `brew install` before starting any AI project that touches documents — infrastructure, not an app.
- **Brand promise:** your files never leave your machine. Zero external connections is a verifiable, load-bearing feature, not a footnote.
- **Playbook:** open-source-to-SaaS (Plausible / Cal.com pattern). AGPLv3 core, free forever for individuals; paid tiers where teams and compliance appear.

## 4. Target users & journeys

### P1 — The developer (launch audience)
Finds Haypile via Hacker News / GitHub. Installs with one command, runs `hay add`, gets a working semantic search over their notes in under two minutes. Adds it to Claude Code / Cursor via MCP with a one-line config. Steady state: forgets it exists; documents are just always searchable.

### P2 — The programmatic user
Building an internal tool or agent. Never uses the CLI beyond setup — hits `POST localhost:11500/api/query` and the MCP endpoint. Their journey is the API docs page. This persona is what makes Haypile infrastructure.

### P3 — The office (law firm, clinic, accountant) — post-v1
Buyer and user are different people. An IT consultant installs Haypile on an office server once. Staff use a browser bookmark: type a question, get an answer with citations, click a citation, see the source document. This persona is served by the future web UI + paid tier, **not** by v1.

## 5. v1 scope

**v1 is CLI + API only. No web UI.** This is the Ollama playbook: ship the best possible engine; let the community build frontends on the API. The UI arrives later (see roadmap) informed by real usage, bundled with the team features businesses pay for.

### 5.1 Commands

```
hay add <folder> [--tag <tag>]   # index a folder and watch it for changes
hay search "<query>" [--tag]     # hybrid retrieval, results with citations
hay ask "<question>"             # RAG answer with cited sources (requires Ollama)
hay list                         # indexed folders, doc counts, index health
hay remove <folder>              # un-index a folder
hay status                       # daemon status, model info, "0 external connections"
hay serve                        # run the daemon (REST API + MCP on localhost:11500)
```

Design rules:
- `hay add` starts indexing immediately with a visible progress bar and prints the next command to try. Time from install to first successful search: **under 2 minutes**.
- Any command auto-starts the daemon if it isn't running. Users never learn the daemon/client distinction.
- **The no-Ollama experience is first-class, not a fallback.** Keyword search (FTS5 + BM25) must be excellent on its own — install → `hay add` → first successful search works with zero dependencies, keeping the 2-minute activation promise honest. When Ollama is detected, semantic search lights up automatically as an upgrade. `hay ask` without Ollama explains what's missing and falls back to `hay search`. Search never requires an LLM.
- Citations (file path + page number) appear in all search and ask output. Non-negotiable.

### 5.2 File formats (v1)

Markdown, plain text, PDF (text-layer), docx. Everything else: skip with a logged warning. (OCR for scanned PDFs, pptx, html → v1.x.)

### 5.3 API surface (v1)

- `POST /api/query` — hybrid search, returns chunks + scores + source metadata
- `POST /api/ask` — RAG answer + citations (503 with clear message if no LLM available)
- `GET /api/sources`, `POST /api/sources`, `DELETE /api/sources` — manage indexed folders
- `GET /api/status` — health, counts, index freshness
- **MCP server** exposing `search` and `ask` tools (stdio + HTTP transport), with copy-paste config snippets for Claude Code, Cursor, and Codex in the README.

Localhost-bind only by default. `--host` flag exists but warns loudly (no auth in v1).

## 6. Architecture

Four subsystems; CLI, REST, and MCP are thin clients over one core engine.

**Ingestion pipeline:** fsnotify watcher → extractor (PDF/docx/md/txt → text + metadata) → chunker (recursive, structure-aware: headings → paragraphs → sentences; ~300–800 tokens, 10–15% overlap; contextual prefix with doc title + section) → embedder.

**Storage:** single SQLite file. FTS5 for keyword index, sqlite-vec for vectors, plus tables for files (content hashes), chunks (with byte offsets into source files — store pointers, not duplicated text), and the embedding cache.

**Query pipeline:** embed query → vector search + FTS5 keyword search in parallel → merge with Reciprocal Rank Fusion → return top chunks with citations → (optional) LLM generation via Ollama for `ask`.

**Interfaces:** cobra CLI, chi HTTP server, MCP handler — all compiled into the one binary.

### 6.1 Performance strategy (the "Cursor-fast" checklist)

1. **Content-addressed embedding cache** — key every embedding by `sha256(chunk_text)`; never embed the same content twice. Biggest win, cheapest to build.
2. **Async worker pool** — watcher pushes changes onto a queue; goroutine pool handles extract/chunk/embed; search stays available during re-index.
3. **Incremental indexing** — per-file content hashes; only changed files are re-processed. Merkle-style directory hashing for instant cold-start freshness checks.
4. **Structure-aware chunking** — split on document structure, never raw character offsets.
5. **Brute-force vector scan is fine for v1** (fast up to hundreds of thousands of vectors, all local); ANN index is a v2 concern.

Target: sub-100ms search latency on a 100k-chunk corpus on ordinary laptop hardware.

## 7. Tech stack

| Concern | Choice |
|---|---|
| Language | Go 1.24+ (single static binary, trivial cross-compile) |
| CLI | cobra |
| HTTP | chi |
| Storage | SQLite via ncruces/go-sqlite3 (WASM/wazero — pure Go, zero CGo; decided at M0). FTS5 via its ext/fts5; vectors at M1 via its ext/vec1 or sqlite-vec ncruces bindings. Kills the CGo cross-compile risk outright; benchmark vs native (mattn + zig cc) at M1 and swap behind the store interface only if the sub-100ms target demands it |
| File watching | fsnotify |
| Embeddings v1 | Ollama HTTP API, default `nomic-embed-text` (decided: most-pulled Ollama embedder, 274MB, 8k context, best size/quality balance); auto-detect, graceful fallback; eval harness also tracks `embeddinggemma` as challenger |
| Embeddings v2 | ONNX Runtime (hugot / onnxruntime_go) with a bundled small model (candidates: all-MiniLM-L6-v2 at 46MB, EmbeddingGemma-300M) — removes the Ollama dependency |
| Extraction | PDF: klippa-app/go-pdfium via its WASM/wazero build (Chrome's PDFium engine — production-grade extraction, no CGo, cross-compiles cleanly; ledongthuc/pdf rejected — text-order/whitespace mangling on real-world PDFs); stdlib zip+XML for docx; goldmark for markdown |
| Releases | goreleaser → GitHub Releases, Homebrew tap, install script |
| Marketing site / docs | Cloudflare Pages (Astro + Starlight) |
| Downloads / installer | Cloudflare R2 + Worker serving `curl haypile.sh \| sh` |
| Future licensing / telemetry | Cloudflare Worker + D1 (license check once, then offline forever) |
| Quality lab | Python (sentence-transformers, eval scripts) for chunking/model experiments; winners get ported to Go |

## 8. Non-goals (v1)

- No web UI in v1 — but the UI roadmap is declared at launch (see §11.1): a minimal bundled local web UI ships free and open in v1.5 for non-technical users; only the team UI is paid
- No auth / multi-user / roles (paid tier, post-v1)
- No cloud sync or hosted service (v2+, Cloudflare-native twin: Vectorize + Workers AI + R2 + Queues)
- No OCR for scanned documents (v1.x)
- No bundled LLM — generation is delegated to Ollama
- No Windows-service polish beyond "the binary runs" (installer polish v1.x)
- No connectors to SaaS sources (Slack, Notion, Gmail) — filesystem only in v1

## 9. Identity & namespace (secured / to secure immediately)

| Asset | Status |
|---|---|
| haypile.sh | **Canonical home** (decided July 2026): site + docs + installer (`curl haypile.sh \| sh`), the Bun playbook. Register at Spaceship (~$31 first year), DNS on Cloudflare — before anything goes public |
| haypile.dev | Optional/defensive (~$11/yr); `haypile.dev/install` was the fallback installer plan — grab before launch if remembered |
| haypile.com | Registered by someone else but dormant (no DNS, July 2026) — acquire via broker later if mainstream traction justifies it |
| haypile.io / haypile.ai | Defensive only, post-traction; skip for now |
| GitHub | `github.com/benydishon/haypile` (decided: personal account for attribution; Go module path matches). Org migration possible later at the cost of an import-path rename |
| Software collisions | None found (July 2026 search) |
| Binary name | `hay` (3 keystrokes; brand ≠ command, like ripgrep→rg) |
| Mascot / lore | A "haypile" is the winter food cache a pika builds — gathers, stores, retrieves. |

Adjacency note: deepset's Haystack is an established RAG framework. Haypile is a clearly different word and product category (ready-to-run tool vs. framework); the shared metaphor signals the category. Accepted trade-off.

## 10. Licensing, trust & business model

Haypile's brand *is* trust. Ollama is the architecture role model **and** the cautionary tale: its MIT core earned years of goodwill, then its closed-source GUI app (developed in a private repo, shipped unlicensed) and quiet cloud-model pivot burned much of it. Haypile adopts Ollama's engine playbook and explicitly rejects its trust mistakes. These commitments are public, in the README, from day one:

### 10.1 License

- **AGPLv3 for the entire core**, public repo from day one, developed fully in the open — no private-repo surprises.
- Why AGPL over MIT: MIT would allow a cloud provider to ship a hosted Haypile without contributing back; AGPL keeps the product free for every actual user while requiring anyone offering it as a service to open-source their changes. Standard choice for OSS-with-a-business (Grafana, Plausible, Cal.com), and it reads as a feature to the target audience.

### 10.2 Trust commitments (non-negotiable, versioned with the code)

1. **The open/paid boundary is declared upfront and never quietly redrawn.** Open forever: indexing, search, ask, REST API, MCP, CLI, and (from v1.5) the basic single-user local web UI — the full single-user product, no feature hostages. Paid: team features (auth, roles, audit logs), the multi-user team web UI, hosted version, commercial licenses. The dividing line is single-user vs. team, not engine vs. interface.
2. **"Zero external connections" is a verifiable feature, not a claim.** `hay status` reports outbound connections (target: 0). No telemetry at launch; if ever added, it is opt-in, documented loudly, and off by default.
3. **No silent network behavior, ever.** Any future update-check or license validation is a single disclosed request; the product works offline forever after. A local-first tool must never phone home to keep working.
4. **All development happens in the public repo.** If a component can't be open (e.g., a future hosted control plane), it is clearly branded and distributed as a separate product, never mixed into the open binary's download path.

### 10.3 Revenue

- **Free forever:** the full local single-user product (AGPLv3).
- **Pro (paid, post-v1):** auth + per-folder access control, multi-user, audit logs (who searched what, when — the compliance feature regulated buyers pay for), team-shared indexes, priority support, the team web UI.
- **Later:** hosted version on the Cloudflare stack for teams that don't want to run a server; commercial licensing for orgs that can't do AGPL.
- **Positioning note:** the trust gap Ollama opened is a market opening — "verifiably local, boringly honest" is a competitive wedge, not just an ethic.

## 11. Milestones

| Milestone | Scope | Definition of done |
|---|---|---|
| **M0 — Walking skeleton** (wk 1) | cobra CLI, `hay add` + `hay search`, naive chunking, SQLite + FTS5 keyword search only | Index a folder of .md/.txt and get ranked keyword results |
| **M1 — Semantic** (wk 2–3) | Ollama embeddings, sqlite-vec, hybrid search + RRF, embedding cache | "agreement cancellation" finds "termination clauses" |
| **M2 — Real documents** (wk 3–4) | PDF + docx extraction, structure-aware chunking, page-number metadata, citations in output | Search a folder of PDFs, results cite file + page |
| **M3 — Daemon** (wk 4–5) | `hay serve`, chi REST API, fsnotify watcher, worker pool, auto-start | Drop a file in a watched folder; it's searchable in seconds |
| **M4 — Ask + MCP** (wk 5–6) | `hay ask` via Ollama with cited sources; MCP server; editor config docs | Claude Code answers a question from local docs through Haypile |
| **M5 — Launch** (wk 6–8) | goreleaser, Homebrew tap, haypile.sh installer, README with demo GIF, eval set, Show HN | Public launch |

Standing habits from M0: (1) an eval set of ~10 queries with expected results, run on every retrieval-affecting change; (2) build-in-public posts per milestone; (3) **testing policy** — `go test ./... -race` + the eval runner, all green before any merge, wired into CI at M0 alongside cross-compilation. Test the engine, not the wiring: chunker, RRF merge, embedding cache, and incremental-indexing logic get thorough table-driven tests; extractors are tested against a `testdata/` corpus of real fixture files (including deliberately nasty PDFs); every CLI command and API endpoint gets happy-path + key-failure integration tests against a temp SQLite db. From M3, one end-to-end daemon smoke test in CI (serve → add folder → wait for index → query API → assert citations) runs under `-race` to catch what mocked concurrency hides. No coverage-percentage chasing on thin CLI wiring.

### 11.1 Post-v1 roadmap (declared publicly in the launch README)

| Version | Scope |
|---|---|
| v1.x | OCR for scanned PDFs, pptx + html extraction, Windows installer polish |
| **v1.5** | **`hay web` — minimal bundled local web UI** (search box → answer with clickable citations → source preview). Free, AGPL, single-user, served by the existing daemon on localhost. This is the non-technical-user entry point; developed fully in the open per §10.2 — explicitly *not* the Ollama private-repo GUI move. Community UIs on the API are encouraged and linked from the README before v1.5 exists. |
| v2 | ONNX-bundled embeddings (no Ollama dependency), ANN index for very large corpora, possible Ollama-style desktop/tray app if v1.5 usage justifies it |
| Pro (post-v1) | Team layer: auth, roles, audit logs, shared indexes, team web UI (paid — see §10.3) |

## 12. Success metrics

- **Activation:** install → first successful search in < 2 minutes (measure via docs feedback / opt-in telemetry later; design for it now).
- **Launch (90 days):** 1,000+ GitHub stars, 3+ community-built frontends or integrations on the API, MCP listing in editor directories.
- **Quality:** eval-set retrieval precision tracked per release; no silent regressions.
- **Business (12 months):** first 10 paying teams on Pro.

## 13. Risks & mitigations

| Risk | Mitigation |
|---|---|
| CGo (sqlite-vec) complicates cross-compilation | Eliminated at M0: WASM SQLite driver (ncruces) is pure Go, so plain GOOS/GOARCH cross-compiles work. Fallback if M1 benchmarks demand native speed: mattn + zig cc, isolated behind the store interface |
| PDF extraction quality — citations are worthless if extracted text is garbage; the riskiest milestone is M2, not M1 | go-pdfium (PDFium = Chrome's PDF engine) instead of naive parsers; extraction-quality cases in the eval set from M2; real-world contract/paper PDFs in the test corpus |
| Go ML ecosystem is thin (embeddings) | v1 delegates to Ollama; ONNX bundling deferred to v2 |
| Solo dev learning Go + RAG simultaneously | Milestone order keeps something working at all times; Python lab for risky experiments |
| Haystack name adjacency | Distinct word, distinct category; monitor, don't pre-optimize |
| Retrieval quality regressions are invisible | Eval set from M0 |
| Scope creep toward UI/features | Non-goals list above is the contract; revisit only after M5 ships |
| Trust erosion via monetization missteps (the Ollama trap) | Trust commitments in §10.2 are versioned with the code; open/paid boundary declared before launch, never redrawn silently |

## 14. Decisions (open questions resolved — July 2026)

1. **Default embedding model: `nomic-embed-text`.** Most-pulled embedding model on Ollama, 274MB, 8,192-token context, best size/quality balance for laptop hardware; `all-MiniLM-L6-v2` is meaningfully weaker on retrieval quality and survives only as an ONNX-bundling candidate for v2 (46MB). The M1 eval harness still runs `embeddinggemma` (300M, strong multilingual MTEB scores) as challenger — swap only if the eval says so.
2. **Chunk defaults: start at ~500 tokens with 10–15% overlap**, structure-aware splitting as specified in §6. This is a knob tuned by the eval harness at M1, not a decision to debate upfront; the eval set is the tie-breaker for all retrieval parameters.
3. **MCP transport: Streamable HTTP first, stdio second.** The MCP spec (2025-11-25) recognizes exactly two transports — stdio and Streamable HTTP; HTTP+SSE is deprecated. Claude Code, Cursor, and Claude Desktop all support Streamable HTTP, and since the daemon already serves HTTP on :11500, this transport is nearly free. Ship a thin stdio proxy binary-mode (`hay mcp-stdio`) at M4 for clients that prefer launching a process.
4. **Telemetry: none at launch, period.** "Zero external connections" and "anonymous usage pings" cannot share a README; the brand is the verifiable claim. Adoption is measured server-side without touching the user's machine: haypile.sh installer download counts (Cloudflare analytics), Homebrew tap analytics, GitHub stars/issues. Revisit only if a future opt-in mechanism can be loudly documented and off by default per §10.2.
