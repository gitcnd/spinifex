# JOURNAL -- Spinifex Outposts-parity + EBS-grade storage (append-only)

## 2026-07-27 03:12 -- PROJECT INSTANTIATION: RESEARCH, MACHINE AUDIT, PLAN
Plan: onboard the engineering_development_methodology docs, deeply research
the Spinifex/Viperblock/Predastore stack and the AWS Outposts + EBS feature
surfaces, audit this machine, and instantiate the working-document system
with a phased plan.

Done:
- Methodology onboarded: all 12 docs + templates read
  (symlinks/FieldX3D/engineering_development_methodology/).
- Spinifex repo explored (subagent, full report retained in chat): SigV4
  gateway -> NATS -> stateless daemons; EC2 registry ~110+ actions
  (spinifex/gateway/ec2.go); EBS actions implemented: Create/Delete/
  Describe/Attach/Detach/ModifyVolume, DescribeVolumeStatus,
  DescribeVolumesModifications, encryption-by-default trio, Create/Delete/
  Describe/CopySnapshot. Only gp3 accepted
  (spinifex/handlers/ec2/volume/service_impl.go). Attach path: viperblockd
  -> nbdkit -> NBD URI -> QMP blockdev-add -> virtio-blk-pci
  (spinifex/vm/volumes.go).
- Viperblock v1.13.0 explored (module cache): 4 KiB blocks, 4 MiB chunks,
  WAL fsync every 200 ms, chunk upload every 30 s
  (viperblock/viperblock.go); writes ACKED FROM MEMORY before WAL fsync;
  guest FLUSH forces WAL flush; NO cross-node replication -- durability
  beyond the local WAL is delegated to the S3 backend. Snapshots are COW
  metadata freezes (viperblock/snapshot.go). Encryption AES-256-GCM exists.
  Published module zip is MISSING the nbd/ plugin source (packaging gap).
- Predastore v1.13.0 explored (module cache): Reed-Solomon (2+1 / 3+2),
  Raft metadata, QUIC shard nodes, AES-256-GCM at rest, fsync-disciplined
  segment writers. Healer/rebalance DOCUMENTED BUT NOT IMPLEMENTED
  (docs/TODO.md). Missing S3 surface: versioning, lifecycle, CORS,
  CopyObject, DeleteObjects, AbortMultipartUpload not wired over HTTP,
  ListParts/ListMultipartUploads.
- Outposts research (aws.amazon.com/outposts/rack/features/, AWS docs):
  racks locally run EC2, EBS + snapshots, S3, ECS, EKS, RDS, ALB,
  ElastiCache*, EMR*, Route 53 Resolver*, IoT Greengrass; local gateway
  (BGP) + CoIP; capacity management. Spinifex exceeds Outposts on local
  IAM/STS/ECR/NLB/Bedrock; lags on RDS, ElastiCache, EMR, Route 53
  Resolver, CloudWatch, LGW/CoIP semantics, outposts.* APIs.
- EBS research (aws.amazon.com/ebs/features/, EBS user guide): parity
  targets = ack-durable replicated writes (io2 99.999% durability),
  volume types with enforced QoS (gp3/io2 256k IOPS), Elastic Volumes live
  modify, incremental snapshots + FSR, multi-attach (io2, 16 instances,
  I/O fencing), KMS encryption, EBS direct APIs.
- Machine audited by probing (see ENVIRONMENT.md): 72-thread Xeon, 188 GiB
  RAM (only ~37 GiB free at audit), nested KVM enabled, /dev/kvm usable,
  Go bootstraps to 1.26.5 via GOTOOLCHAIN=auto (gateway package built
  clean), sudo needs password, nbdkit/OVN absent on host, host distro
  unsupported by install scripts.
- Working documents instantiated: STATUS.md, JOURNAL.md, DECISIONS.md
  (forks F0-F5), PHASE_GATES.md (Phases -1..5), ENVIRONMENT.md.

Failed/learned:
- "Fix predastore to be a true EBS alternative" is ambiguous: Predastore
  is the S3 layer; Viperblock is the EBS layer. Filed as MAJOR fork F1,
  leaning "harden the whole stack"; needs human ack before Phase 1.
- The nbd/ directory referenced by Viperblock's Makefile is absent from
  the v1.13.0 module zip -- work on the NBD path requires the git clone,
  not the module cache.
- readme.md (lowercase) does not exist in this repo; the file is README.md.

Metrics: gateway-package build ~52 s cold (includes toolchain+module
downloads). Full `go build ./...` launched in background this session;
result to be recorded in the next entry.

Fork movement: F0 opened (OPEN), F1 opened (LEANING a), F2 opened (OPEN),
F3 opened (LEANING KVM guests), F4 opened (LEANING botocore+golden),
F5 opened (LEANING ceph/s3-tests).

Next: get human answers on F0/F1 and the NEEDS HUMAN resource items; then
Phase -1 starting with P-1.1 (baseline unit+integration on this machine).

## 2026-07-27 03:25 -- addendum: full build green
Plan: record the background full-build result.
Done:
- `GOTOOLCHAIN=auto go build ./...` at commit 220141b1: exit 0 (~90 s cold
  including module downloads). /proc/mdstat confirmed no assembled arrays
  (the NVMe isw_raid_member drives are idle).
Failed/learned: none.
Metrics: full build ~90 s cold on 72-thread host.
Fork movement: none.
Next: as above (P-1.1 unit/integration baseline).
