# STATUS -- Spinifex Outposts-parity + EBS-grade storage (dashboard)

<!-- Overwrite-in-place. History lives in JOURNAL.md. -->

Last updated: 2026-07-27 12:55 (F2 deferred by human; P-1.3 CLOSED --
strace proves guest FLUSH never fsyncs; s3-tests baseline running;
F7 vhost-user-blk design note written)
Current phase: Phase -1 (de-risk and baseline), 4/9 [x], 3/9 [~]
Current slice: power-loss mechanism proven at syscall level (zero
fsyncs across 50 explicit NBD FLUSHes; viperblock commit a40cc27) --
P-1.3 closed via open gate restructure, loss-counting rig displaced to
P1.4 (fix verification). s3-tests 838-test baseline running in
background. F7 design decided on paper: pure-Go vhost-user-blk in
viperblockd, amd64-first (F7_VHOST_USER_BLK_DESIGN_NOTE.md).

## NEEDS HUMAN

Nothing blocking. Non-blocking queue:
1. REVIEW: F0/F1 closed by your delegation (DECISIONS.md) -- Outposts
   parity = service-set first then thin outposts.* API; "predastore as
   EBS" = harden the whole Viperblock+Predastore stack. Object if wrong.
1a. ACK REQUESTED (MAJOR fork F2): adopt synchronous peer WAL
   replication as the acked-write durability mechanism. Evidence: it
   costs +3 us over the local fsync we need anyway; the alternative
   (per-write predastore PUTs) measured 14-52x worse (DECISIONS.md F2).
   A "go" here unlocks Phase 1 implementation.
2. RESOLVED 2026-07-27 07:45: human granted the token read+write on
   gitcnd/{spinifex,viperblock,predastore}; all storage branches + the
   v1.13.0 tag pushed to gitcnd/viperblock and gitcnd/predastore.
   NOTE (standing, low priority): these are PLAIN repos, not GitHub
   forks, so they cannot open PRs against mulgadc/* directly. If
   upstreaming becomes the goal: delete them, click Fork on the mulgadc
   repos, re-add to token, and I re-push (identical history, 2 minutes).
3. RESOLVED 2026-07-27: the wildcard-DNS quirk was fixed by the human;
   verified (nonexistent hostnames now NXDOMAIN). services/viperblockd
   now passes normally, CONFIRMING its DNS attribution. However
   spinifex/daemon TestClusterManager_TLSServesHTTPS STILL fails on the
   host (3/3) -- my original DNS attribution for that one was WRONG
   (corrected in the P-1.1 gate note). Cause unknown; standalone TLS
   replica is 2 ms, test passes in netns. Open triage item (non-blocking;
   suspect the daemon TestMain fixtures' interaction with the host
   network). May be an upstream-reportable flake once root-caused.
4. Root access still only needed later for bridged VM networking.
5. RAM budget assumption stands: VMs capped at 16 GiB.
6. NVMe pair: destructive use permitted? Needed by the perf phases.
7. Optional: throwaway real AWS account for golden captures (fork F4).

## Branch map (fork F6)

- spinifex (pushed to fork gitcnd/spinifex):
  - project-management: working documents (this folder).
  - feat/ebs-volume-types-and-snapshot-api, feat/ebs-qos-enforcement,
    feat/outposts-service-parity: created, no commits yet.
- ../viperblock (pushed to gitcnd/viperblock):
  - feat/crash-consistency-harness: P-1.3 harness + 25-cycle evidence
    (commits 8723789, 40b9ef8). ACTIVE.
  - feat/replicated-wal-durability: Phase 1 core (awaits F2 evidence).
  - (planned when F7 evidence lands): feat/data-path-vhost-user-blk or
    feat/data-path-ublk, per gate P-1.9.
- ../predastore (pushed to gitcnd/predastore):
  - feat/shard-healer-and-read-repair: availability probe + P-1.6
    baseline (commit dbe31cf). ACTIVE.
  - feat/s3-api-surface-completion.

## What landed 2026-07-27 (3rd drop): Phase -1 first evidence
- P-1.1 [x]: unit suite green as union of normal + hermetic-netns modes
  (every failure environment-attributed: wildcard DNS vs missing real
  interfaces); integration tier ok in 12.178 s. JOURNAL entry 3.
- P-1.3 [~, SIGKILL leg done]: crash harness (viperblock public API only,
  production open/recover sequence) -- 25 kill cycles, 249635 acked
  writes, 136 flush barriers, 0 flushed writes lost, 0 corrupt, 35401
  acked-unflushed lost = 14.2% memory-ack window. Remaining: power-loss
  leg (VM/QMP), because code reading shows vb.Flush() does NOT fsync (only
  the 200 ms background syncer does) -- guest fsync likely not power-loss
  durable; headline Phase 1 fix target.
- Lesson banked: the harness initially FALSELY accused viperblock of
  losing flushed writes -- cross-session flush accounting bug in the
  harness's own parser, found via discriminating experiments (copy-verify,
  pause-writes, no-replay probe, backend md5 diff). JOURNAL entry 4.

## What landed 2026-07-27 (2nd drop): autonomy wiring
- F0/F1 decided by delegation; F6 git workflow decided; 4 branches pushed
  to gitcnd/spinifex; siblings cloned at v1.13.0 with local branches.

## What landed 2026-07-27 (1st drop): project instantiation
- Research, machine audit, phased plan, fork registry. JOURNAL entry 1.

## Next action
1. P-1.3 power-loss leg: QEMU guest (user networking, /usr/libexec/
   qemu-kvm) with the crash-harness writer on an NBD-served volume
   (the new Go NBD server works for this); QMP quit mid-write; verify.
   Proves/disproves the Flush-no-fsync power-loss window.
2. P-1.7: fetch ceph/s3-tests, baseline against the loopback cluster.
3. F7 endgame planning: vhost-user-blk backend design note (protocol
   handshake, virtqueue handling, memory-region mapping in Go or a
   thin Rust sidecar) -- decide build shape before writing code; pair
   with engine write-lock work (writes regress at depth 16).
2. P-1.3 power-loss leg: QEMU guest (user networking, /usr/libexec/
   qemu-kvm) running the writer against an NBD-served volume; QMP quit
   mid-write; verify. Proves/disproves the Flush-no-fsync loss window
   (code reading says vb.Flush does not fsync -- 200 ms syncer only).
3. Human acks queued: F2 (peer replication -- see NEEDS HUMAN 1a).
4. P-1.7: fetch ceph/s3-tests, baseline against the loopback cluster.
5. When opening Phase 2: extend its gates per the P-1.6 implication --
   failure detection, read shortcutting, degraded/quorum writes, and
   bucket-metadata failover, in addition to the healer (JOURNAL entry 5).
6. Phase 1 scope note (from the P-1.5 baseline): the drain-stall problem
   (writes blocked for minutes under sustained load) joins durability as
   a Phase 1 target -- likely bounded write-buffer + async drain tuning.
7. Backlog: golden-response harness design (P0.3); P-1.6 wiped-store
   variant when healer work starts.
8. BUG backlog (found by the P-1.5 rig, belongs on the durability
   branch): nbdkit-plugin reconnect after an unclean client disconnect
   triggers WAL recovery that fails on a missing local checkpoints
   directory ("open .../checkpoints/blocks.00000000.bin: no such file")
   leaving the volume unserveable for that connection. Reproduce: seed
   fresh base_dir with createvol-s3, connect+disconnect qemu-img bench
   twice. Also: qemu reconnect robustness deserves its own test.

## Gate snapshot
Phase -1: 1/8 [x] (P-1.1) + 2/8 [~] (P-1.3 SIGKILL leg, P-1.6
availability leg) . P0: 0/4 . P1: 0/5 . P2: 0/5 . P3: 0/6 . P4: 0/6 .
P5: 0/4

## Standing reminders
- ALL go commands: GOTOOLCHAIN=auto. Baselines additionally GOWORK=off.
- Unit suites: normal mode EXCEPT daemon + services/viperblockd which
  need hermetic netns (`unshare -r -n` + lo up) -- wildcard-DNS trap.
- NEVER print or commit the .env token; source .env inline per push.
- Feature branches cut from main (220141b1), not from project-management.
- Sibling repo work stays on local branches until the human forks.
- Shared, loaded machine: VMs <= 16 GiB, kill orphans, nothing
  destructive without explicit ask.
- No validation artifact, no claim.
