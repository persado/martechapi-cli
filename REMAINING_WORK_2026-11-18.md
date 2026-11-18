# Remaining work, 2026-11-18

Open items after release 0.1.3. This file replaces every earlier `REMAINING_WORK_*` file; finished items were removed.

| Item | Why it is open |
|---|---|
| Decide whether `main` should require the `ci` status check before merge | Branch protection requires a pull request only. |
| Test `--edit` in a real Windows terminal (PowerShell, Windows Terminal) | The Bubble Tea editor was tested only through its model, piped input and tmux on macOS. |
| Review `httpTimeout` (2 min per exchange) against slow links | A 10 MiB upload needs at least 85 KB/s to finish; the server allows 10 min. A per-command timeout for asset transfers may be needed. |
| Confirm on the server whether `alwaysUse` terms must appear in the original variant | The spec is ambiguous; the CLI does not check it locally. |
| Staging test assets expire on their own | The API has no delete operation. `smoke.txt` expires 2026-12-17; `ux-smoke.txt`, `brief.txt` and the `asset-check` file expire by 2026-11-19. |
