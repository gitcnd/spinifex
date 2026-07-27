# STATUS -- Spinifex Outposts-parity + EBS-grade storage (dashboard)

<!-- Overwrite-in-place. History lives in JOURNAL.md. -->

Last updated: 2026-07-27 06:40 (Phase -1: P-1.1 [x]; P-1.3 SIGKILL leg
done, 0 flushed-write losses in 25 crash cycles; P-1.6 baseline measured:
predastore single-node outage = reads 100% available but ~2320x slower,
writes 0% available)
Current phase: Phase -1 (de-risk and baseline), 1/8 [x], 2/8 [~]
Current slice: predastore availability probe + single-node-kill baseline
on branch feat/shard-healer-and-read-repair (commit dbe31cf): healthy
read 200/200 in 0.82 s; one node killed -> read 200/200 in 1907.56 s
(dead-node timeouts every read), PUTs 0/10, CreateBucket fails. Phase 2
scope must include failure detection + degraded writes + metadata
failover, not just the healer.

## NEEDS HUMAN

Nothing blocking. Non-blocking queue:
1. REVIEW: F0/F1 closed by your delegation (DECISIONS.md) -- Outposts
   parity = service-set first then thin outposts.* API; "predastore as
   EBS" = harden the whole Viperblock+Predastore stack. Object if wrong.
2. Fork mulgadc/viperblock and mulgadc/predastore to your account and add
   them to the token's scope, so the storage branches (which now carry
   real work) can be pushed. Until then they are local-only at
   ../viperblock and ../predastore.
3. FYI, a machine quirk you may want to fix globally: the resolver's
   emsvr.com search domain wildcard-resolves EVERY nonexistent hostname to
   91.103.1.84. It broke two test packages (triaged; hermetic-netns
   workaround in ENVIRONMENT.md).
4. Root access still only needed later for bridged VM networking.
5. RAM budget assumption stands: VMs capped at 16 GiB.
6. NVMe pair: destructive use permitted? Needed by the perf phases.
7. Optional: throwaway real AWS account for golden captures (fork F4).

## Branch map (fork F6)

- spinifex (pushed to fork gitcnd/spinifex):
  - project-management: working documents (this folder).
  - feat/ebs-volume-types-and-snapshot-api, feat/ebs-qos-enforcement,
    feat/outposts-service-parity: created, no commits yet.
- ../viperblock (LOCAL-ONLY until forked):
  - feat/crash-consistency-harness: P-1.3 harness + 25-cycle evidence
    (commits 8723789, 40b9ef8). ACTIVE.
  - feat/replicated-wal-durability: Phase 1 core (awaits F2 evidence).
- ../predastore (LOCAL-ONLY until forked):
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
1. P-1.4: WAL-replication latency prototype (informs MAJOR fork F2):
   measure (b) peer-process replication vs (c) per-write small-object PUT
   to a local predastore dev server (3node-loopback recipe in
   ENVIRONMENT.md); compare against a P-1.5 engine-level baseline.
2. P-1.3 power-loss leg: QEMU guest (user networking, /usr/libexec/
   qemu-kvm) running the writer against an NBD-served volume; QMP quit
   mid-write; verify. Proves/disproves the Flush-no-fsync loss window
   (code reading says vb.Flush does not fsync -- 200 ms syncer only).
3. When opening Phase 2: extend its gates per the P-1.6 implication --
   failure detection, read shortcutting, degraded/quorum writes, and
   bucket-metadata failover, in addition to the healer (JOURNAL entry 5).
4. P-1.5: NBD-path fio baseline (belongs with the VM deployment work);
   engine-level interim numbers acceptable if labeled as such.
5. P-1.7: fetch ceph/s3-tests, baseline against the loopback cluster.
6. Backlog: golden-response harness design (P0.3); P-1.6 wiped-store
   variant when healer work starts.

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
