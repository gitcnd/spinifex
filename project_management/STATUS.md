# STATUS -- Spinifex Outposts-parity + EBS-grade storage (dashboard)

<!-- Overwrite-in-place. History lives in JOURNAL.md. -->

Last updated: 2026-07-28 10:15 (new agent resumed after the IDE crash:
state verified, regression runner 6/6 green, F7 ACKED -> DECIDED,
OPS-1 no-op per human; production engine wiring behind vhost-user
starting)
Current phase: Phase -1 nearly done (6/9 [x], 1/9 [~]) + Phase 1 OPENED
(P1.1 [~], P1.4 [~])
Current slice: F7 production wiring -- real viperblock WAL engine behind
the vhost-user-blk backend (serve mode), guest boot, in-guest fio.

## NEEDS HUMAN

Nothing blocking. Non-blocking queue:
1. REVIEW (standing): F0/F1 were closed by delegated judgment call
   (DECISIONS.md) -- Outposts parity = service-set first then thin
   outposts.* API; "predastore as EBS" = harden the whole
   Viperblock+Predastore stack. Object if wrong.
2. NVMe pair (2x 1.9 TB, idle isw_raid members): destructive use
   permitted? Needed by the realistic-performance phases.
3. Optional: throwaway real AWS account for golden captures (fork F4).
4. Root access only needed later for bridged VM networking (P-1.2 can
   start with user networking).
5. Standing, low priority: gitcnd/{viperblock,predastore} are plain
   repos, not GitHub forks, so they cannot open PRs against mulgadc/*.
   If upstreaming becomes the goal: delete them, click Fork on the
   mulgadc repos, re-add to token, re-push (identical history, 2 min).
RESOLVED 2026-07-28: F7 ACKED by the human (verbatim in DECISIONS.md
F7) -- vhost-user-blk is the VM data path. F2 was acked earlier the
same day. OPS-1 (token rotation) resolved as NO-OP per human direction
(fork-scoped token, revoked after the work, merge-back reviewed).

## Security audit (2026-07-27, read-only, forked chat)
Preliminary hardening review at
project_management/SECURITY_AUDIT_PRELIMINARY.md (no code changed);
remediation queue inside. Three findings have committed reproducers
(SP-2 IAM condition drop, VB-1 close-after-failed-drain data loss,
VB-4 secret-key logging). OPS-1 resolved as no-op (see above). VB-1
overlaps Phase 1 durability work and should be fixed on that branch.

## Branch map (fork F6)

- spinifex (pushed to fork gitcnd/spinifex):
  - project-management: working documents (this folder).
  - feat/ebs-volume-types-and-snapshot-api, feat/ebs-qos-enforcement,
    feat/outposts-service-parity: created, no commits yet (Phase 3/4).
- ../viperblock (pushed to gitcnd/viperblock):
  - feat/data-path-vhost-user-blk: F7 winner -- pure-Go vhost-user-blk
    backend, boots real QEMU, ~1.8x NBD in-guest. ACTIVE (production
    engine wiring).
  - feat/replicated-wal-durability: Phase 1 -- SyncOnFlush barrier +
    walrepl peer replication (both landed, suite green).
  - feat/crash-consistency-harness: P-1.3 harness + strace evidence +
    perfbench (P-1.5).
  - feat/data-path-native-nbd-server: P-1.9 candidate (a), measured.
  - fix/nbd-close-open-race: lifecycle mutex fix (stands on mechanism
    + 70 clean post-fix cycles; differential repro closed 0/40).
- ../predastore (pushed to gitcnd/predastore):
  - feat/shard-healer-and-read-repair: availability probe + P-1.6
    baseline + s3-tests rig + P-1.7 frozen baseline. ACTIVE for Phase 2.
  - feat/s3-api-surface-completion: created, no commits yet (Phase 2).

## What landed 2026-07-28 (5th drop): resume + F7 decided
- IDE-crash handoff consumed: state re-verified (all branches pushed,
  local==fork byte-identical, trees clean, no orphans), regression
  runner 6/6 green in 6m23s (spinifex-unit 67s, daemon-hermetic 26s,
  integration 2s, viperblock 192s, predastore 82s, crashharness 14s).
- F7 MAJOR fork DECIDED on human ack; OPS-1 no-op per human direction;
  STATUS drift fixed; temp_resume_handoff.md retired (content here);
  stale /tmp/vb-main-worktree pruned.

## What landed 2026-07-28 (4th drop): F7 evidence + Phase 1 slices
- P-1.9 [x]: vhost-user-blk boots real QEMU 7.2 + Debian 13 guest to
  SSH; /dev/vdb 16 MiB round-trip byte-identical; in-guest fio, same
  raw backend: vhost 28.5k randwrite IOPS vs nbdkit 15.7k (~1.8x),
  reads ~1.35x. Bugs fixed en route: bootindex, SET_VRING_NUM parse.
- P1.4 [~]: SyncOnFlush durable barrier landed + strace-verified on the
  served path (24 fsyncs across 20 barriers where pre-fix code
  produced zero across 50 flushes).
- P1.1 [~]: walrepl synchronous peer replication wired into the flush
  barrier; replica-WAL-byte-identical test green; fail-closed barrier.
- P-1.7 [x]: s3-tests baseline frozen 123/621/94 (Phase 2 floor).

## Next action
1. IN PROGRESS: wire the REAL viperblock WAL engine (not the raw-file
   demo engine) behind the vhost-user backend as a serve mode; boot the
   guest against it; re-run in-guest fio. Watch DECISIONS F7 qualifier
   1: the engine REGRESSES writes at depth (global WAL write lock) --
   transport alone will not show the win end to end; measure and
   record honestly, then pair with engine write-concurrency work.
2. F2 next slice: promotion/recovery from replica WALs, reconnect/
   resync, degraded-mode policy; then close the acked-but-unflushed
   memory window (14.2% measured) via replicate-on-WriteAt or
   WAL-on-ack.
3. Phase 2 (predastore) when opened: healer + read-repair + degraded/
   quorum writes + metadata failover (P-1.6: 1 node down = writes 0%,
   reads 100% but ~2320x slower); then the S3 surface gaps (CopyObject,
   DeleteObjects, AbortMultipartUpload over HTTP, versioning).
4. Backlog: P-1.2 single-node VM deploy (rig VM ready at
   ../vm-images/rig-node1.qcow2, key rig_ssh_key); volatile-cache FUSE
   loss-count rig (P1.4 remainder); P0.3 golden-response harness;
   drain-stall fix (Phase 1 target, from P-1.5: writes stall minutes
   under sustained load); s3-tests subset suite into the regression
   runner (P0.2 remainder); VB-1 security fix on the durability branch.

## Gate snapshot
Phase -1: 6/9 [x] (P-1.1, P-1.3, P-1.4, P-1.5, P-1.7, P-1.9) + 1/9 [~]
(P-1.6) + open: P-1.2, P-1.8 . P0: 0/4 ticked (P0.2 runner exists,
validated 6/6 today; s3-tests subset still missing) . P1: 2/5 [~]
(P1.1, P1.4) . P2: 0/5 . P3: 0/6 . P4: 0/6 . P5: 0/4

## Standing reminders
- ALL go commands: GOTOOLCHAIN=auto. Baselines additionally GOWORK=off.
- GOFIPS140=v1.0.0 for ANY binary embedding viperblock (fipsboot panic
  otherwise).
- Unit suites: normal mode EXCEPT spinifex/daemon which needs hermetic
  netns (`unshare -r -n` + lo up) -- TLS-test stall, cause under triage.
- NEVER print or commit the .env token; source .env inline per push.
- Feature branches cut from main (220141b1), not from project-management.
- Shared, loaded machine: VMs <= 16 GiB, kill orphans (pgrep -x, never
  pkill -f with self-matching patterns), nothing destructive without
  explicit ask.
- One-command regression: project_management/tools/run_regressions.sh.
- Predastore dev cluster without root: see ENVIRONMENT.md traps
  (pd-shim + SSL_CERT_FILE recipe).
- No validation artifact, no claim.
