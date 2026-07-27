# DECISIONS -- Spinifex Outposts-parity + EBS-grade storage (fork registry)

Decision protocol: forks move to DECIDED only with evidence (a measured
number, a prototype result, a benchmark, or human ack). MAJOR forks
additionally require explicit human ack, recorded verbatim with date.

## F0 -- What "feature parity with AWS Outposts" means  [MAJOR]
Status: DECIDED (2026-07-27, by delegated judgment call -- human wrote
  "please can you check that the plan is strong enough to let you work on
  this with maximum autonomy, and lets go!" without contesting the
  recommendation; flagged for review, revisit on any human comment)
Decision: option (c) -- service-set parity first (RDS being the largest
  item), thin outposts.* metadata API layer afterwards.
Options:
  (a) SERVICE-SET parity: Spinifex locally offers what an Outposts rack
      offers locally -- EC2, EBS + snapshots, S3, ECS, EKS, RDS, ALB,
      ElastiCache, EMR, Route 53 Resolver, VPC extension, local gateway
      (BGP) + customer-owned IP (CoIP) semantics, capacity management.
      Spinifex already exceeds Outposts in some areas (local IAM/STS, ECR,
      Bedrock, NLB) and lags in others (RDS, ElastiCache, EMR, Route 53
      Resolver, CloudWatch, LGW/CoIP, Outposts capacity APIs).
  (b) LITERAL API parity: additionally implement the outposts.* API surface
      (ListOutposts, GetOutpost, GetOutpostInstanceTypes, capacity tasks,
      sites) so tooling written for Outposts management works unchanged.
  (c) Both, phased: (a) first, (b) as a thin metadata layer later.
Evidence: gap analysis from repo exploration (2026-07-27, see JOURNAL entry
  1) and AWS Outposts docs (aws.amazon.com/outposts/rack/features/,
  docs.aws.amazon.com/outposts/latest/userguide/what-is-outposts.html).
Revisit trigger: human answer; re-examine after Phase 0 when the e2e
  harness shows real AWS-tooling behavior against Spinifex.
Escalation path: default to (c) if the human delegates the call.

## F1 -- What "fix predastore to be a true EBS alternative" means  [MAJOR]
Status: DECIDED (2026-07-27, by delegated judgment call -- same delegation
  as F0; flagged for review)
Decision: option (a) -- harden the whole Viperblock + Predastore stack to
  EBS-grade durability and features.
Context: in Spinifex, PREDASTORE is the S3-compatible object store;
  VIPERBLOCK is the EBS-alternative block layer that uses Predastore as its
  durable backing store. "Predastore as an EBS alternative" is therefore
  ambiguous.
Options:
  (a) Harden the whole block-storage stack (Viperblock + Predastore
      together) to EBS-grade durability and features: replicated/durable
      acknowledged writes, automatic shard repair (healer), volume QoS,
      full snapshot/volume API surface. This is the reading consistent with
      the codebase.
  (b) Turn Predastore itself into a block store (replace Viperblock).
      Not recommended: discards a working, well-tested WAL/chunk engine.
Evidence: architecture findings 2026-07-27 (JOURNAL entry 1): Viperblock
  has NO cross-node replication of the hot layer -- acknowledged writes
  live in memory, WAL fsync is periodic (200 ms), chunks upload to S3 every
  30 s; Predastore has erasure coding but its healer/rebalance is
  documented-not-implemented (predastore docs/TODO.md).
Revisit trigger: human answer.
Escalation path: proceed under (a) if delegated.

## F2 -- Durability architecture for acknowledged writes  [MAJOR]
Status: LEANING (b) synchronous peer WAL replication (2026-07-27, on
  P-1.4 prototype numbers below; MAJOR, so human ack still required to
  move to DECIDED)
Decision: (pending human ack) option (b): ack = local WAL write +
  synchronous replication to a peer node's WAL (group-commit fsync on
  both), promote-on-failure. Option (c) per-write predastore PUTs is
  REJECTED on evidence; option (a) local-fsync-only remains an interim
  single-node mode.
Evidence (P-1.4, viperblock proto/f2walrepl commit f7d5ca1, results/
  2026-07-27_f2_durability_latency.txt): same-run 4 KiB acked-write p50:
  local fsync 5627 us; peer-replicated (TCP + group commit) 5630 us --
  replication adds ~3 us over the fsync it must pay anyway (localhost;
  add ~0.1-0.5 ms for a real LAN). Per-write predastore PUT: 77643 us
  p50 at concurrency 1, 293463 us at 16, ~50 ops/s ceiling -- 14-52x
  worse and two orders of magnitude short on throughput.
2026-07-27 DEFERRAL: human deferred the ack ("the people we need to ask
  are asleep"). Status stays LEANING (b); Phase 1 implementation waits;
  all other work proceeds (P-1.3 power-loss evidence, P-1.7, F7 design
  landed during the deferral window).
Options considered:
  (a) Ack-after-local-WAL-fsync (single node): cheapest; still loses the
      node's un-uploaded writes on host loss. AWS EBS acks only after
      intra-AZ replication, so this alone is not parity.
  (b) Synchronous WAL replication to a peer node (primary/replica pairs,
      promote on failure). Closest to real EBS architecture.
  (c) Quorum write of WAL records/small chunks straight to Predastore
      (leverages existing RS coding; latency risk -- per-write S3 PUT).
Revisit trigger: real-LAN measurement when a second machine exists (the
  localhost RTT caveat); and if implementation shows the replica cannot
  keep up with sustained write bursts, revisit group-commit windows
  before revisiting the architecture.
Escalation path: (a) as a degraded single-node mode, clearly labeled;
  (c) stays rejected unless predastore ever grows a sub-millisecond
  small-object path.

## F3 -- Development environment on this host  [minor]
Status: LEANING KVM guests
Decision: run the Spinifex dev cluster as Debian 13 KVM guests on this
  Oracle Linux host (nested virt verified enabled), 1-3 nodes; unit and
  integration tests run directly on the host with GOTOOLCHAIN=auto.
Evidence: host distro unsupported by install scripts; /dev/kvm usable
  without root; nested = Y; gateway package builds on host (2026-07-27).
Revisit trigger: if VM overhead or RAM pressure (only ~37 GiB free at
  audit) blocks e2e work, ask the human for dedicated hardware or memory.
Escalation path: containers (podman) for control-plane-only testing.

## F4 -- Oracle strategy for AWS API parity  [minor]
Status: LEANING botocore models + real-AWS golden captures
Decision: use (1) the machine-readable botocore/AWS SDK service models as
  the shape/validation oracle, (2) the real AWS CLI + Terraform AWS
  provider as the client-behavior oracle, and (3) optionally recorded
  golden responses from a real AWS account for semantic comparison.
  LocalStack/moto are NOT oracles (they are reimplementations with their
  own bugs).
Evidence: standard practice; zero-cost items (1) and (2) already implied by
  the repo's own e2e suites which drive the AWS CLI.
Revisit trigger: if a semantic dispute cannot be settled from docs/models,
  a small real-AWS account becomes necessary (NEEDS HUMAN, costs money).

## F5 -- S3 correctness oracle for Predastore  [minor]
Status: LEANING ceph/s3-tests
Decision: adopt the ceph/s3-tests compatibility suite as the external
  oracle for Predastore's S3 surface; measure the baseline pass-rate before
  setting the gate number.
Evidence: none yet (Phase -1 gate P-1.6 produces the baseline).
Revisit trigger: if the suite's AWS-specific assumptions produce excessive
  false failures, curate a documented exclusion list in the open.

## F6 -- Repo, branch, and push workflow  [minor]
Status: DECIDED (2026-07-27)
Decision: one branch per feature-gap component, per the human's explicit
  request ("When identifying feature-gaps to plug, please create branches
  for each of those components"). Spinifex work pushes to the fork
  github.com/gitcnd/spinifex using the .env fine-grained token (verified
  push+admin). The working documents live on branch `project-management`;
  feature branches cut from `main` (= upstream 220141b1) so they stay
  clean for potential upstream PRs. Viperblock and Predastore are cloned
  as siblings (../viperblock, ../predastore) from mulgadc with LOCAL-ONLY
  branches until the human forks them (the token cannot: fork API returns
  "Bad credentials" -- scoped to gitcnd/spinifex only). Local wiring uses
  an untracked go.work, NOT go.mod replace directives (clone-deps.sh's
  approach would dirty the fork's go.mod). Commit at every validated
  milestone with numbers in the message; push at session milestones.
Evidence: token probes 2026-07-27 (GET /user 200; gitcnd/spinifex
  push:true admin:true; POST /repos/mulgadc/*/forks -> Bad credentials).
Revisit trigger: human forks the two sibling repos or widens the token.

## F7 -- Guest block-device data path (replace nbdkit)  [MAJOR]
Status: OPEN (human-initiated 2026-07-27: "nbdkit is a bottleneck - that
  needs to be replaced with a block device driver")
Decision: (pending measurement)
Context: today guest virtio-blk -> QEMU NBD client -> socket -> nbdkit
  (C shim) -> Go plugin -> viperblock. Every IO pays protocol framing,
  extra copies, and a process hop. Candidates, cheapest first:
  (a) Native Go NBD server inside viperblockd (drop nbdkit, keep NBD):
      removes the extra process + C boundary; multi-conn + Unix socket.
      Low risk, modest gain; also fixes the nbd/-missing-from-module-zip
      packaging gap.
  (b) vhost-user-blk backend around the viperblock engine; QEMU
      vhost-user-blk-pci hands the guest's virtio queues to our daemon
      over shared memory, bypassing QEMU's block layer and all sockets
      (the SPDK pattern). Biggest VM-path gain; medium effort.
  (c) ublk (io_uring userspace block driver, kernel 6.x): viperblock
      serves a real /dev/ublkbN; uniquely enables HOST-side attach
      (format/mount volumes on the node), guest path still goes through
      QEMU block layer.
  (d) NVMe-oF TCP target: standard kernel initiators, multi-queue +
      multipath for free; most protocol work; strongest multi-node
      attach story (relevant to EBS multi-attach later).
Evidence: none yet. Required before LEANING: P-1.5 baseline split into
  engine-level vs NBD-path fio numbers (quantify what nbdkit actually
  costs), then a prototype of (b) and/or (c) measured against it
  (gate P-1.9). Note (a) can proceed as a low-risk step regardless of
  the endgame choice if the baseline confirms the nbdkit tax.
Revisit trigger: P-1.9 numbers; also revisit if QEMU/kernel versions on
  target deployments (Debian 13) constrain (b)/(c).
Escalation path: (b) vhost-user-blk is the presumptive endgame for VM
  IO; (c) ublk complements it for host-side attach rather than
  competing. A native QEMU block driver in C (cgo into viperblock) was
  considered and rejected upfront: highest maintenance burden against
  QEMU internals for no advantage over (b).
2026-07-27 EVIDENCE (P-1.5(ii), viperblock commit 5c0155f): the nbdkit
  tax is real and measured -- 2.5-3.8x 4 KiB IOPS loss vs direct file
  with nbdkit's own C plugin; production path writes 15.2k IOPS d1 where
  the engine acks in ~5 us. TWO qualifiers the decision must respect:
  (1) viperblock writes REGRESS with queue depth (12.1k IOPS at d16 vs
  15.2k at d1 -- engine write-lock contention), so transport replacement
  alone will not deliver its win without engine concurrency work;
  (2) depth-1 reads are backend-dominated (2.28 ms/op with predastore
  chunk GETs), so the read path's leverage is cache/backend, not
  transport. Fork stays OPEN pending P-1.9 candidate prototypes.
