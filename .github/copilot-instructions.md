# GitHub Copilot instructions

This repository is a Go Packer provisioner with embedded Windows PowerShell 5.1 guest scripts.

## Repository guidance

Read and follow [AGENTS.md](../AGENTS.md) for the architecture, coding standards and skill index, and [INVARIANTS.md](../INVARIANTS.md) for the hard constraints and source documentation rules. Load the owning module's guide before editing its files.

Use the relevant shared workflow linked from AGENTS.md for Go, guest PowerShell, code review, releases, baseline updates or WinRM protection. Read its file explicitly when native skill discovery is unavailable; do not assume a Markdown link has already loaded its instructions.

## Validation

Use the current [Makefile](../Makefile) recipes, including `make check` before a commit. Report actual results and unavailable checks. Passing simulated communicator or local PowerShell tests does not verify a real WinRM build.
