# Remaining work, 2026-11-18

Open items after the 0.1.3 API commands (PR from `feature/api-commands`).

| Item | Why it is open |
|---|---|
| Test `--edit` in a real Windows terminal (PowerShell, Windows Terminal) | The Bubble Tea editor was tested only through its model and piped input on macOS. |
| Review `httpTimeout` (2 min per exchange) against slow links | A 10 MiB upload needs at least 85 KB/s to finish; the server allows 10 min. A per-command timeout for asset transfers may be needed. |
| Clean up staging assets created by smoke tests (`smoke.txt`, `ux-smoke.txt`, and `brief.txt` from the user manual) | The API has no delete operation; they expire after 30 days and 1 day; `brief.txt` after 1 day. |
| Confirm on the server whether `alwaysUse` terms must appear in the original variant | The spec is ambiguous; the CLI does not check it locally. |
