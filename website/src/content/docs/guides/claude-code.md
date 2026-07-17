---
title: Use Haypile from Claude Code
description: Give AI coding agents your local documents as a knowledge source over MCP.
---

Haypile's daemon speaks MCP (Model Context Protocol), the standard way to expose tools to AI agents. Once connected, Claude Code can search your indexed documents whenever a question calls for it: "what does our Meridian contract say about termination?" becomes answerable inside your editor, grounded in your actual files, with citations.

## Connect Claude Code

One command:

```sh
claude mcp add --transport http haypile http://localhost:11500/mcp
```

That is the whole setup. Claude Code now sees two tools:

- `search_documents`: hybrid search over everything you have indexed, returning cited passages
- `list_sources`: what folders are indexed, so the agent knows what it can search

Make sure something is indexed (`hay add ~/Documents`) and the daemon is running. It starts automatically on `hay add` and stays up.

## Per-project setup with hay init

For a project or case folder, `hay init` writes a `.mcp.json` in the folder:

```sh
cd ~/cases/acme-litigation
hay init
```

Claude Code picks up `.mcp.json` automatically when opened in that folder. Anyone who clones or opens the project gets the integration without running the `claude mcp add` command themselves.

## Cursor and other editors

Any MCP client that supports the Streamable HTTP transport can use the same endpoint: `http://localhost:11500/mcp`. For clients that prefer launching a process (stdio transport), configure:

```json
{
  "command": "hay",
  "args": ["mcp-stdio"]
}
```

`hay mcp-stdio` bridges stdio to the daemon and auto-starts it if needed.

## A privacy note worth understanding

Haypile itself makes zero external connections; that does not change here. But an MCP client is a separate program with its own behavior: when Claude Code calls `search_documents`, the passages that come back become part of Claude's context, which is sent to Anthropic like the rest of your conversation. Connecting an AI agent to your documents is a choice about that agent, not a change in what Haypile does. For fully offline question answering, use `hay ask` with a local LLM instead.

## Try it

Open a Claude Code session in a folder with an indexed project and ask something only your documents can answer:

> what did we agree with the vendor about payment terms?

Claude will call `search_documents`, read the cited passages, and answer from them. The citations flow through, so you can verify against the source file.
