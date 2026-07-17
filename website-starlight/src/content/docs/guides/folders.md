---
title: Configure a folder
description: Per-folder tags and exclude patterns with hay init and .haypile.yml.
---

Folders you work in deserve more than a bare `hay add`: a tag for scoped search, patterns for files that should never be indexed, and editor wiring. `hay init` sets all of that up, and a plain YAML file keeps it adjustable.

## Run the wizard

```sh
cd ~/cases/acme-litigation
hay init
```

```
Setting up /Users/you/cases/acme-litigation
Tag for filtered search [acme-litigation]:
Exclude patterns, comma-separated [none]: drafts/**, *.bak
Make these docs available to AI tools here (Claude Code, Cursor)? [Y/n]
Wrote .haypile.yml
Wrote .mcp.json (Claude Code will pick it up in this folder)
Indexed 214 files (1,892 chunks), 0 unchanged.
```

Three questions, each with a sensible default. If no local LLM is detected it also offers `hay llm setup` at the end.

For scripts and dotfiles, skip the questions entirely:

```sh
hay init --yes --tag acme --exclude "drafts/**,*.bak"
```

## The config file

`hay init` writes `.haypile.yml` in the folder. It is short enough to read in full:

```yaml
tag: acme-litigation
exclude:
  - drafts/**
  - "*.bak"
```

- `tag` scopes searches: `hay search "deposition" --tag acme-litigation`
- `exclude` takes gitignore-style glob patterns, matched against paths relative to the folder. `drafts/**` skips a subtree; `*.bak` skips matching files at any depth.

## Edit it anytime, by hand

The config file is the source of truth and the daemon watches it. Add an exclude pattern, save, and the matching files leave the index within seconds. Remove the pattern and they come back. No re-add, no restart, no command to remember.

This also means the config travels with the folder: sync it, commit it to a repo, or copy it to another machine, and the same rules apply wherever Haypile indexes that folder.

## What init does and does not do

`hay init` is a writer for the config plus the normal indexing you would get from `hay add`. It does not create a separate index or a different kind of source. A folder set up with `init` and a folder added with `add` behave identically afterwards; `init` just gives you the tag, excludes, and `.mcp.json` in one pass.
