<!--
Thanks for contributing to Haypile. Please read CONTRIBUTING.md first if you
have not. A few quick checks below keep review fast.
-->

## What this changes

<!-- A short description of the change and why. Link the issue it addresses. -->

Closes #

## Checklist

- [ ] `gofmt -l .` prints nothing, `go vet ./...` and `go test ./... -race` pass
- [ ] Every commit is signed off (`git commit -s`), per CONTRIBUTING.md
- [ ] If retrieval is affected, the eval set in `eval/` still passes
- [ ] This change does not add a network call, open a non-local listener, or
      introduce telemetry (if it does, it was discussed in an issue first)
