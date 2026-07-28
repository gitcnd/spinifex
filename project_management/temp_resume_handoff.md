# HANDOFF -- resume the Spinifex Outposts-parity + EBS-grade-storage work

Audience: the agent picking this up after an IDE crash on 2026-07-28
~09:17 UTC. Everything below happened BEFORE your snapshot; the working
tree and all pushed branches are intact. Do NOT redo completed work.
This file is a pointer + orientation; the AUTHORITATIVE state lives in
the four working documents in this folder -- read them first:

1. `project_management/STATUS.md`   <- read this FIRST; it is the dashboard
2. `project_management/JOURNAL.md`  <- append-only history, newest at top
3. `project_management/DECISIONS.md`<- fork registry F0-F7 (why things are)
4. `project_management/PHASE_GATES.md` <- gates + evidence, marks [ ]/[~]/[x]
5. `project_management/ENVIRONMENT.md`  <- machine audit + TRAPS (read traps!)

## What this project is (one paragraph)

Two goals from the human: (1) make Spinifex feature-comparable to AWS
Outposts (service-set parity first, thin `outposts.*` API later -- fork
F0), and (2) make its storage a "true EBS alternative" by hardening the
Viperblock (block) + Predastore (S3) stack to EBS-grade durability and
features (fork F1). We follow the engineering methodology in
`symlinks/FieldX3D/engineering_development_methodology/` (gates before
code, no-claim-without-artifact, honest partials, decision forks, bank
every lesson). We are in Phase -1 (de-risk/baseline) mostly complete,
with Phase 1 (durability) opened.

## Repos, branches, remotes (VERIFIED 2026-07-28 09:17)

Three sibling clones under `/home/devnull/Downloads/github/`:
- `spinifex/`   remote `origin` = https://github.com/gitcnd/spinifex
- `viperblock/` remote `fork`   = https://github.com/gitcnd/viperblock
- `predastore/` remote `fork`   = https://github.com/gitcnd/predastore
An untracked `go.work` wires all three (use `.`, `../predastore`,
`../viperblock`). The human's token is in `spinifex/.env` (gitignored;
NEVER print/commit it). Push recipe is in ENVIRONMENT.md "Git auth".

Current checked-out branches / HEADs:
- spinifex   `project-management` @ 31127d66 (pushed to origin)
- viperblock `feat/data-path-vhost-user-blk` @ a2fc287 (pushed to fork)
- predastore `feat/shard-healer-and-read-repair` @ 72c6269 (pushed to fork)

Working trees are CLEAN (only this untracked handoff file in spinifex).
No orphaned processes (qemu/nbdkit/s3d all 0, verified).

Branch-per-component (fork F6). Spinifex feature branches
(feat/ebs-volume-types-and-snapshot-api, feat/ebs-qos-enforcement,
feat/outposts-service-parity) are created but empty. Viperblock branches:
feat/crash-consistency-harness (P-1.3 harness + strace + flush-barrier
evidence), feat/replicated-wal-durability (P1.4 SyncOnFlush + F2 walrepl),
feat/data-path-native-nbd-server (P-1.9 candidate a), fix/nbd-close-open-race
(lifecycle mutex), feat/data-path-vhost-user-blk (F7 winner). Predastore:
feat/shard-healer-and-read-repair (availability probe + s3-tests rig),
feat/s3-api-surface-completion (empty).

## Standing rules that will bite you if ignored (from ENVIRONMENT.md)

- ALL go commands: prefix `GOTOOLCHAIN=auto` (host go is 1.21; repo needs
  1.26.5). For baseline runs also `GOWORK=off` so released deps are used,
  not the local sibling clones.
- Build the nbdkit plugin / any viperblock-embedding binary with
  `GOFIPS140=v1.0.0` or the fipsboot guard panics at runtime.
- Unit suites: spinifex/daemon + services/viperblockd need a hermetic
  netns on this host (`unshare -r -n sh -c 'ip link set lo up; <cmd>'`)
  OR they pass in normal mode now that the human fixed the wildcard DNS
  (retest; the regression runner already splits them).
- Predastore dev cluster without root: `PATH=/tmp/pd-shim:$PATH
  SSL_CERT_FILE=/tmp/predastore/server.pem ./scripts/start.sh -w
  3node-loopback` (config/3node-loopback.toml). Delete
  /tmp/predastore/server.pem when changing host IPs.
- TRAP that cost real time repeatedly: do NOT `pkill -f <pattern>` when
  the pattern matches your own shell command line, and do NOT rely on a
  captured `$!` as an nbdkit/qemu pid (the launcher subshell's pid != the
  daemon's). Use `pgrep -x`/pidfiles and verify with `pgrep -x` after.
  Also: multi-line `-m` commit messages inside `&&` chains silently
  no-op here -- commit with a single-line `-m`, or run commit standalone.
- One-command regression runner: `project_management/tools/run_regressions.sh`
  (6 suites; predastore suite auto-skips if a dev cluster is up).

## Gate scoreboard (see PHASE_GATES.md for evidence notes)

Phase -1: P-1.1 [x] baseline, P-1.3 [x] crash-consistency (SIGKILL + the
strace proof that guest FLUSH pre-fix never fsynced), P-1.4/P-1.5 [x]
(nbdkit tax + engine baseline), P-1.6 [~] predastore node-kill baseline,
P-1.7 [x] s3-tests baseline (123 pass / 621 fail / 94 skip -- the Phase 2
floor), P-1.9 [x] data-path decision. P-1.2 (single-node VM deploy) and
P-1.8 (human review) still open.
Phase 1: P1.1 [~] (F2 replication transport landed), P1.4 [~] (durable
flush barrier landed + strace-verified). P1.2/P1.3/P1.5 open.

## Decisions locked (DECISIONS.md)

- F0 service-set-parity-first, F1 harden-whole-stack: DECIDED (delegated).
- F2 synchronous peer WAL replication: DECIDED (human ack verbatim
  2026-07-28). Two slices landed on feat/replicated-wal-durability:
  SyncOnFlush durable barrier + the walrepl package (byte-exact replica
  WAL, fail-closed barrier).
- F7 guest data path: LEANING (b) vhost-user-blk on DECISIVE evidence,
  **awaiting human ack** (MAJOR). This is the top NEEDS-HUMAN item.

## THE most recent win (context for the next step)

vhost-user-blk (fork F7) now works end-to-end on real QEMU 7.2 + Debian
13 guest: guest boots to SSH, /dev/vdb round-trips 16 MiB byte-identical,
and in-guest fio (same raw backend, three paths in one guest) gives
vhost-user 28.5k randwrite IOPS vs nbdkit 15.7k vs plain virtio 14.3k
(~1.8x on writes). Artifact:
viperblock/vhostuser/results/2026-07-28_inguest_fio_vhost_vs_nbd_vs_plain.txt.
Two bugs fixed to get there (both committed): boot-order (need
`bootindex=0` on the OS disk or SeaBIOS hangs on the blank vhost disk),
and SET_VRING_NUM/BASE parsed as `struct{u32 index; u32 num}` (num at
offset 4, not a bare u64 -- the low bits were the index=0, so the queue
never started and guest I/O hung). The working QEMU launch line is in
JOURNAL 2026-07-28 09:10 and reproducible via
viperblock `vhostuser/cmd/vhost-user-blk-serve --raw-file`.

## NEXT ACTIONS (in priority order -- also in STATUS.md "Next action")

1. NEEDS HUMAN, non-blocking: get the F7 ack (adopt vhost-user-blk).
   While waiting, everything below is unblocked.
2. Wire the REAL viperblock WAL engine (not the raw-file demo engine)
   behind the vhost-user backend as a serve mode; boot the guest against
   it; re-run in-guest fio. Watch qualifier 1 in DECISIONS F7: the engine
   REGRESSES writes at depth (global WAL write lock) -- pair transport
   with engine write-concurrency work or the win won't show end to end.
3. F2 next slice: promotion/recovery from replica WALs, reconnect/resync,
   degraded-mode policy; and close the acked-but-unflushed memory window
   (14.2% measured) via replicate-on-WriteAt or WAL-on-ack.
4. Phase 2 (predastore) when opened: healer + read-repair + degraded
   writes + metadata failover (P-1.6 showed 1 node down = writes 0%,
   reads 100% but ~2320x slower); then the missing S3 surface that
   blocked s3-tests (CopyObject, DeleteObjects, AbortMultipartUpload over
   HTTP, versioning) -- see the P-1.7 failure breakdown.
5. Backlog: P-1.2 single-node VM deploy (the rig VM at
   ../vm-images/rig-node1.qcow2, ssh key rig_ssh_key, is ready);
   volatile-cache FUSE loss-count rig for P1.4; the nbd-race long-drain
   differential is closed (0/40 pre-fix; fix stands on mechanism).

## How to re-orient in 5 minutes

Read STATUS.md, then the top 3 JOURNAL entries, then run
`project_management/tools/run_regressions.sh` to confirm a green baseline
before building anything. If something environmental fails, check
ENVIRONMENT.md "## Traps" FIRST. Then pick action 2 above.
