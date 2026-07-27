# F7 design note: vhost-user-blk backend for viperblock

Status: DESIGN (2026-07-27). No code yet; this note exists so the build
starts from a decided shape (fork F7, gate P-1.9 remainder).

## Why this and not something cheaper (the measured chain)

- nbdkit + NBD costs 2.5-3.8x on 4 KiB IOPS even with nbdkit's native C
  plugin (P-1.5(ii), viperblock tests/perfbench/results/).
- A native Go NBD server lands WITHIN NOISE of nbdkit (P-1.9 candidate
  (a)): the cost is the NBD request/reply round trip (2 socket syscalls
  + wakeups per op), which every NBD server pays.
- Therefore the only way to a step-change is removing the per-op socket
  round trip: shared-memory virtio rings, i.e. vhost-user-blk. This is
  the SPDK/cloud-hypervisor-established pattern.

## Architecture

    guest virtio-blk driver
      -> virtqueues in GUEST RAM (shared with us via memfd mmap)
      -> our daemon polls/receives kicks (eventfd), parses descriptors,
         calls viperblock engine (ReadAt/WriteAt/Flush) directly,
         writes used-ring entries, signals irqfd
    QEMU: -device vhost-user-blk-pci,chardev=... with
          -object memory-backend-memfd,share=on -numa node,memdev=...
    Control plane: vhost-user protocol over a unix socket (feature
    negotiation, memory-region table, vring addresses, kick/call fds).

## Build-shape decision

Options considered:
- (i) Pure Go in viperblockd: mmap + eventfd via syscall; virtio ring
  structs parsed directly. The engine is Go, so IO stays in-process --
  no extra hop. x86-64 memory model + Go atomics cover the
  barrier requirements; document this as an amd64-first implementation.
- (ii) Rust sidecar (vhost-user-backend crate): mature protocol code,
  but the engine is Go, so every IO would cross ANOTHER IPC boundary --
  reintroducing exactly the tax we are removing. Rejected.
- (iii) C libvhost-user embedded via cgo: protocol maturity, but cgo
  per-op callbacks are the nbdkit-Go-plugin pattern again. Rejected.

DECISION SHAPE: (i) pure Go, amd64 first. Escalation path if the
protocol surface proves too fiddly: (iii) for the CONTROL PLANE only
(rings still parsed in Go; libvhost-user handles negotiation).

## Scope for the P-1.9 prototype (thin slice)

1. Single queue, no multiqueue; VIRTIO_BLK_T_IN / T_OUT / T_FLUSH only.
2. vhost-user messages: GET/SET_FEATURES, GET/SET_PROTOCOL_FEATURES,
   SET_OWNER, SET_MEM_TABLE, SET_VRING_{NUM,ADDR,BASE,KICK,CALL,ENABLE},
   GET_QUEUE_NUM, GET/SET_CONFIG (capacity). Reconnect: OUT OF SCOPE for
   the prototype (restart guest instead); REQUIRED before production.
3. Bench: same qemu-img... no -- qemu-img cannot drive vhost-user (it is
   a guest device). Prototype bench needs a minimal guest: Debian
   cloud image + fio, OR qemu with a tiny initramfs running fio. Budget
   a session for the guest rig alone (it also serves P-1.3's future
   volatile-cache verification and the P-1.2 dev deployment).
4. Gate numbers (P-1.9): fio 4k randwrite/randread d1/d16 in-guest for
   virtio-blk-over-NBD(nbdkit) vs vhost-user-blk, same volume config.

## Interactions and constraints

- FLUSH mapping: T_FLUSH must become a DURABLE barrier (Phase 1 P1.4,
  the strace finding: today's Flush never fsyncs). Land durability
  semantics with, not after, the transport change.
- Engine concurrency: writes REGRESS with depth (12.1k IOPS d16 vs
  15.2k d1) on the global write-buffer lock; a parallel transport
  without engine sharding will hit the same wall. Pair the work.
- Spinifex integration: vm/volumes.go + the ebs.mount NATS flow swap an
  NBD URI for a vhost-user socket path + memfd memory-backend flags on
  the QEMU command line; needs a daemon-restart-safe socket lifecycle.
- Host-side attach (volumes without a VM) stays NBD or later ublk; the
  Go NBD server (candidate a) remains useful there and for the
  crash/power harnesses.

## Prior art to read before coding

- QEMU docs: docs/interop/vhost-user.rst (protocol), vhost-user-blk
  device; SPDK vhost_blk target; cloud-hypervisor vhost_user_block;
  virtio 1.2 spec sections 2.6-2.7 (split virtqueue) and 5.2 (block).
