# Go standards

Use the Go version in [go.mod](../../go.mod), the owning module's `AGENTS.md`, and [INVARIANTS.md](../../INVARIANTS.md), especially I15-I19.

- Keep policy in Go and reuse the Packer communicator boundary. See the [architecture overview](../architecture/overview.md) for package ownership.
- Propagate context, bound retries and make file, timer and goroutine ownership explicit. Preserve unknown facts instead of permissive defaults.
- Wrap errors when callers need the underlying cause; handle them at the responsible boundary.
- Follow I19 for documentation, including unexported functions and tests. Regenerate HCL configuration when source fields change.

Use [go-development](../skills/go-development/SKILL.md) for implementation and [testing](testing.md) for verification.
