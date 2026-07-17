# Security Policy

Haypile is a local-first, private document tool, so its privacy properties are
the whole point. Reports of vulnerabilities are very welcome.

## Reporting a vulnerability

Please **do not** open a public issue for security problems. Instead, use
GitHub's private vulnerability reporting ("Report a vulnerability" under the
repository's [Security tab](https://github.com/BenyD/haypile/security)), or
email the maintainer.

Include enough detail to reproduce: affected version or commit, the scenario,
and the impact. You will get an acknowledgement as quickly as possible.

## What Haypile protects

Haypile indexes your documents into a single SQLite database on your disk and
answers questions about them locally. The embedding model ships inside the
binary, and the only listener is a daemon bound to `localhost`. Nothing about
your documents or your usage is meant to leave the machine.

**In scope:** anything that would let document contents, the index, or usage
data leave the machine unexpectedly; any way to reach the local daemon (REST or
MCP on `localhost:11500`) from off-host or from another user on the same
machine; path traversal or injection through indexed file paths, folder
config (`.haypile.yml`), or query input; a network call Haypile makes that the
user did not initiate; and supply-chain issues in the build that could smuggle
any of the above into a release.

## What Haypile does not defend against (by design)

- A compromised local machine. If malware can read your disk, it can read your
  documents and the index directly, with or without Haypile.
- The contents you deliberately send to your own LLM endpoint. `hay ask`
  forwards retrieved passages to the OpenAI-compatible server **you** chose and
  run (Ollama, LM Studio, llama.cpp, Jan). What that server does with them is
  governed by that server, not Haypile.
- Another local process reading Haypile's files with your own user permissions.
  The index has the same protection as any other file in your home directory.

## Commitments the code keeps

- The embedding model is bundled; semantic search makes no network calls.
- The daemon listens on `localhost` only, never on a public interface.
- `hay status` reports the outbound connection count, and the target is zero.
- There is no telemetry. If that ever changes it will be opt-in, documented
  loudly, and off by default.
