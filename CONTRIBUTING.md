# Contributing to Haypile

Thanks for your interest in Haypile. It is a small, deliberately-scoped
project, so contributions are welcome but please read this first, especially
the [Licensing](#licensing-of-contributions) and
[Sign-off](#sign-off-developer-certificate-of-origin) sections, which apply to
every contribution.

## Before you start

- For anything non-trivial, **open an issue first** to discuss the change.
  Haypile is intentionally minimal and tightly scoped, so a quick conversation
  avoids wasted work on something that does not fit.
- Small fixes (typos, docs, obvious bugs) can go straight to a PR.

## Development

The full setup lives in the [README](./README.md#development). In short:

```sh
./hack/fetch-model.sh    # one-time: download the embedding model weights
go build ./cmd/hay       # build the binary
```

Before opening a PR, please make sure these pass:

```sh
gofmt -l .               # must print nothing
go vet ./...
go test ./... -race      # green before any merge
```

If your change affects retrieval quality, run the eval set in [`eval/`](./eval/);
it holds a query set with expected results and is the guard against silent
regressions in search.

## The privacy contract is load-bearing

Haypile's entire value is that nothing leaves your machine. The
[trust commitments](./README.md#trust-commitments) in the README are versioned
with the code and are not decorative:

- The embedding model ships inside the binary. Search makes **no network calls**.
- The only listener is the local daemon, and it binds to `localhost` only.
- `hay status` reports outbound connections, and the target is **zero**. There
  is no telemetry.

Any change that adds a network call, opens a non-local listener, introduces
telemetry, or otherwise lets document data or usage data off the machine
**must** be discussed in an issue first, and must keep these commitments
intact. `hay ask` talking to a local, user-run LLM endpoint is the one outbound
call Haypile makes, and it is to a server the user chose and controls. Nothing
else should reach the network.

If you believe you have found a security vulnerability, **do not** open a public
issue or PR. Follow the disclosure process in [`SECURITY.md`](./SECURITY.md).

## Licensing of contributions

Haypile is licensed under [**AGPL-3.0-or-later**](./LICENSE). Two things apply
to every contribution you submit:

1. **Inbound = outbound.** Your contributions are provided under the same
   license as the project, AGPL-3.0-or-later.

2. **Commercial-relicensing grant.** You **retain copyright** to your
   contribution. In addition to the AGPL license above, you grant **Beny Dishon
   K (the project maintainer, [@BenyD](https://github.com/BenyD))** a perpetual,
   worldwide, non-exclusive, royalty-free, irrevocable license to use,
   reproduce, modify, sublicense, and distribute your contribution, including
   the right to relicense it under **other terms, including proprietary or
   commercial licenses**, as part of Haypile or a derivative of it.

Why this exists: it keeps the door open for the future paid team layer
described in the README roadmap without having to track down and re-license
every past contribution. It does not take away your rights. You keep your
copyright and can use your own contribution however you like.

## Sign-off (Developer Certificate of Origin)

Haypile uses the [Developer Certificate of Origin](https://developercertificate.org/)
(DCO). It is a lightweight way for you to certify that you wrote, or otherwise
have the right to submit, the code you are contributing. There is no separate
paperwork or account signup.

To sign off, add the `-s` flag when you commit:

```sh
git commit -s -m "Your commit message"
```

This appends a line to your commit message:

```
Signed-off-by: Your Name <your.email@example.com>
```

Use your real name and an email you can be reached at. **Every commit in a pull
request must be signed off**, and CI enforces this. If you forget, you can
retroactively sign off the commits on your branch with:

```sh
git rebase --signoff main
git push --force-with-lease
```

By signing off, you certify the following:

```
Developer Certificate of Origin
Version 1.1

Copyright (C) 2004, 2006 The Linux Foundation and its contributors.

Everyone is permitted to copy and distribute verbatim copies of this
license document, but changing it is not allowed.


Developer's Certificate of Origin 1.1

By making a contribution to this project, I certify that:

(a) The contribution was created in whole or in part by me and I
    have the right to submit it under the open source license
    indicated in the file; or

(b) The contribution is based upon previous work that, to the best
    of my knowledge, is covered under an appropriate open source
    license and I have the right under that license to submit that
    work with modifications, whether created in whole or in part
    by me, under the same open source license (unless I am
    permitted to submit under a different license), as indicated
    in the file; or

(c) The contribution was provided directly to me by some other
    person who certified (a), (b) or (c) and I have not modified
    it.

(d) I understand and agree that this project and the contribution
    are public and that a record of the contribution (including all
    personal information I submit with it, including my sign-off) is
    maintained indefinitely and may be redistributed consistent with
    this project or the open source license(s) involved.
```
