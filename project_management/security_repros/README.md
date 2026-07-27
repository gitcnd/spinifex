# Security-finding runtime reproducers

Standalone Go programs that reproduce specific findings from
`../SECURITY_AUDIT_PRELIMINARY.md` at runtime, against our own code. They
are DEFENSIVE checks (confirm-then-fix); each prints a CONFIRMED / NOT
CONFIRMED line.

Run (from this directory):

    GOFLAGS=-mod=mod GOTOOLCHAIN=auto go run ./sp2_condition
    GOFLAGS=-mod=mod GOTOOLCHAIN=auto go run ./vb1_close_dataloss
    GOFLAGS=-mod=mod GOTOOLCHAIN=auto go run ./vb4_secret_log

Each is isolated: file backend / in-memory only, no network, no
predastore cluster, no ports. `vb1_close_dataloss` writes under
`/tmp/vb1_repro`.

CAVEAT: `go.mod` uses `replace` directives pointing at the sibling clones
at absolute paths (`/home/devnull/Downloads/github/{viperblock,predastore}`).
Adjust those paths for another checkout. Pinned require versions are
v1.13.0.

Coverage:
- sp2_condition   -> SP-2 (IAM Condition blocks silently dropped -> over-grant)
- vb1_close_dataloss -> VB-1 (Close deletes local WAL after a failed chunk
  upload -> loss of a flushed block) [CRITICAL]
- vb4_secret_log  -> VB-4 (S3 SecretKey/AccessKey rendered into logs)

These three were the runtime pass on 2026-07-27. Remaining findings in
the audit are still static-only and want their own reproducers before fix.
