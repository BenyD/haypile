---
title: Configuration
description: The .haypile.yml per-folder config file and the data directory.
---

Haypile needs no configuration to work. When you want control, there are exactly two places to look: a per-folder YAML file and a handful of environment variables.

## .haypile.yml

Lives in a source folder, written by `hay init`, editable by hand. The daemon watches it: saving a change re-syncs the index within seconds.

```yaml
tag: acme-litigation
exclude:
  - drafts/**
  - "*.bak"
  - "**/archive/**"
```

### tag

Applied to everything indexed under this folder. Search with `--tag` to scope results. A tag passed explicitly to `hay add --tag` wins over the config.

### exclude

Glob patterns matched against paths relative to the folder, gitignore flavored:

| Pattern | Matches |
| --- | --- |
| `drafts/**` | everything under the `drafts` subtree |
| `*.bak` | `.bak` files at any depth (bare names match everywhere) |
| `**/archive/**` | anything under any directory named `archive` |

Adding a pattern removes already-indexed matching files from the index on the next sync. Removing a pattern brings them back. A malformed pattern or broken YAML fails the indexing pass loudly rather than silently indexing everything.

## The data directory

Everything Haypile stores lives in one place:

| File | What it is |
| --- | --- |
| `~/.haypile/haypile.db` | The entire index: files, chunks, vectors, FTS. One SQLite file |
| `~/.haypile/daemon.json` | Runtime file: the running daemon's address and pid |

Set `HAYPILE_DIR` to relocate it. Deleting the directory deletes the index and nothing else; your documents are never touched. Back it up by copying one file.

## Environment variables

See the [CLI reference](/reference/cli/#environment-variables) for the full table. The two most useful:

- `HAYPILE_DIR`: keep separate indexes (for tests, for work vs personal) by pointing this at different directories.
- `HAYPILE_NO_DAEMON=1`: force direct index access with no background process, useful in scripts and CI.
