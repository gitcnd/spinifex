# STATUS -- Spinifex Outposts-parity + EBS-grade storage (dashboard)

<!-- Overwrite-in-place. History lives in JOURNAL.md. -->

Last updated: 2026-07-27 05:10 (autonomy granted; scope forks F0/F1 decided
by delegation; git workflow wired; Phase -1 execution starting)
Current phase: Phase -1 (de-risk and baseline), 0/8 boxes
Current slice: project instantiation + git/branch workflow (fork F6);
P-1.1 baseline test run in progress.

## NEEDS HUMAN

Nothing blocking. Non-blocking queue:
1. REVIEW: F0 and F1 were closed by delegated judgment call (DECISIONS.md)
   -- Outposts parity = service-set first then thin outposts.* API;
   "predastore as EBS" = harden the whole Viperblock+Predastore stack.
   Say the word if either reading is wrong.
2. Fork mulgadc/viperblock and mulgadc/predastore to your account and give
   the token access (it currently reaches only gitcnd/spinifex), so the
   storage branches can be pushed off this machine. Until then they are
   local-only clones at ../viperblock and ../predastore.
3. Root access (password sudo) when we reach cluster-in-VM work: nbdkit +
   guest tooling inside VMs avoids this, but bridged networking will
   eventually want it. /usr/libexec/qemu-kvm + user-mode networking is the
   no-root interim path.
4. RAM budget: host had ~37 GiB free at audit. I will cap any VM at 16 GiB
   and single VM at a time unless told otherwise.
5. The two 1.9 TB NVMe drives (idle Intel-RAID members): destructive use
   permitted? Not needed until performance phases.
6. Optional: throwaway real AWS account for golden captures (fork F4).

## Branch map (fork F6)

- spinifex (push = fork gitcnd/spinifex):
  - project-management: working documents (this folder), harness scripts.
  - feat/ebs-volume-types-and-snapshot-api: EC2/EBS API surface gaps.
  - feat/ebs-qos-enforcement: IOPS/throughput enforcement for volume types.
  - feat/outposts-service-parity: Phase 4 service work (RDS et al).
- ../viperblock (local-only until forked):
  - feat/crash-consistency-harness: P-1.3 harness.
  - feat/replicated-wal-durability: Phase 1 core (per F2 outcome).
- ../predastore (local-only until forked):
  - feat/shard-healer-and-read-repair: Phase 2 core.
  - feat/s3-api-surface-completion: CopyObject, DeleteObjects, Abort etc.

## What landed 2026-07-27 (2nd drop): autonomy wiring
- F0/F1 DECIDED by delegation; F6 (git workflow) decided with token probes
  as evidence; .env token verified (push+admin on gitcnd/spinifex only).
- Working docs to be committed on branch project-management and pushed.

## What landed 2026-07-27 (1st drop): project instantiation
- Research, machine audit, phased plan, fork registry. JOURNAL entry 1.

## Next action
1. P-1.1: `GOWORK=off GOTOOLCHAIN=auto make test` baseline on host; then
   `make test-integration`; triage failures in writing; freeze results.
2. Clone siblings, create branch map above, wire go.work (untracked).
3. P-1.3 design: crash-consistency harness against viperblock as a plain
   host process (no root needed): writer with verifiable pattern ->
   kill -9 -> recover -> replay-verify. Lives on
   viperblock:feat/crash-consistency-harness.
4. P-1.4 prototype: WAL replication latency measurements (informs F2).
5. P-1.5: fio-style perf baseline (can start with viperblock's own bench
   tooling on host; NBD-path numbers once nbdkit exists in a VM).
6. P-1.7: fetch ceph/s3-tests, run against a host-local predastore dev
   server, record baseline.
7. Deferred pending resources: P-1.2 (single-node VM deployment), P-1.6
   (3-node predastore cluster -- can run as 3 host processes, try that).

## Gate snapshot
Phase -1: 0/8 . P0: 0/4 . P1: 0/5 . P2: 0/5 . P3: 0/6 . P4: 0/6 . P5: 0/4

## Standing reminders
- Run ALL go commands with GOTOOLCHAIN=auto. Use GOWORK=off for baseline
  runs that must see the released v1.13.0 module deps, not local clones.
- NEVER print or commit the .env token; .env is gitignored -- keep it so.
- Push spinifex branches with the inline credential helper (see
  ENVIRONMENT.md "Git auth"); sibling repos are local-only for now.
- Feature branches cut from main (220141b1), NOT from project-management,
  so upstream PRs stay clean.
- Host distro unsupported by install scripts: cluster work in Debian 13
  guests only (qemu-kvm at /usr/libexec/qemu-kvm, user networking).
- Shared, loaded machine: cap VM RAM at 16 GiB, kill orphans at session
  end, nothing destructive without explicit ask.
- No validation artifact, no claim.
