# Database decisions for the side project

Choosing between SQLite and Postgres for the bookkeeping tool.

## Why SQLite won

Single user, single machine, no server to run. Enabling WAL mode (write-ahead
logging) means reads never block on writes, which covers the only concurrency
we actually have: the importer writing while the UI reads.

Backups are one file copy. The whole "database administration" story
disappears.

## What we give up

No connection pooling to tune because there are no connections — but also no
row-level locking, so bulk imports should batch writes in a single
transaction. Measured: 50k rows insert in 1.2s batched vs 41s unbatched.

## Revisit when

If this ever becomes multi-user over a network, Postgres is the default
answer; SQLite over NFS is a known footgun.
