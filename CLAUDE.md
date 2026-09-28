# timeperformance-mcp

Single binary exposing the TimePerformance API v4 via two interfaces:
- **MCP stdio server** (JSON-RPC over stdin/stdout) — default mode.
- **CLI subcommands** (composable via Bash) — `tp-mcp <command> [flags]`.

## Build & Deploy

```bash
make build     # builds ./tp-mcp (local)
make install   # builds directly to /usr/local/bin/tp-mcp
make test      # go test ./...
```

**Always use `make install`** after changes — MCP clients load the binary from
`/usr/local/bin/`.

## Environment Variables

- `TIMEPERFORMANCE_URL` — base URL (optional, default `https://pma.timeperformance.com`)
- `TIMEPERFORMANCE_LOGIN` / `TIMEPERFORMANCE_PASSWORD` — API credentials (required)
- `TIMEPERFORMANCE_AUTO_WRITE` — `1` bypasses the elicitation confirmation
- `TIMEPERFORMANCE_MIN_INTERVAL_MS` — local pacing between calls (default 1000)

`TP_*` short aliases are accepted for all of them.

Credentials come from the web app: profile/Settings gives **read-only** user
credentials (GET only), Administration → API BackOffice gives read/write
back-office credentials. Writes with user credentials return HTTP 403 by design.

## Architecture

```
cmd/tp-mcp/            → Entry point, routes args[0] → MCP server or CLI
internal/
  ├── timeperformance/ → REST client (client.go), API methods (api.go), types (types.go)
  ├── tools/           → MCP tool registrations + exported Format* helpers
  └── cli/             → CLI dispatcher (cli.go reads, write.go writes)
```

Routing in `cmd/tp-mcp/main.go`:
- no args, `mcp`, or `serve` → `server.ServeStdio`
- `help` / `version` → handled before the client is built (no credentials needed)
- any other first arg → `cli.Run`

Flags and positional args can be given in any order: `flags.reorder` moves
positionals last before handing them to the stdlib `flag` package.

## Conventions

- **One tool per file group by domain**: `projects.go`, `tasks.go`, `wbs.go`
  (deliverables + phases share the `WBSElement` shape), `expenses.go`,
  `users.go`, `time.go`, `portfolios.go`, `admin.go`.
- **Write payloads use pointer fields** (`*string`, `*int`, `*bool`) so a PATCH
  only carries what the caller actually set. In MCP tools that distinction comes
  from `args` helpers in `helpers.go` (raw arguments map); in the CLI it comes
  from `flags.optStr/optInt/optBool` (backed by `FlagSet.Visit`).
- **Reports are passthrough JSON**: `get_project_report` and
  `get_portfolio_progress_report` return the API's own tree via `FormatJSON`
  rather than mirroring dozens of report schemas in Go.
- **Names resolve to ids** through `ResolveProjectID`, `ResolvePortfolioID` and
  `ResolveUserID` (`me` resolves through `/whoami`).
- **Rate limits**: the client holds a mutex for the whole request and paces
  calls (1/s), retrying `429` with `Retry-After`. Do not add concurrency.

## Human-in-the-loop (write tools)

`internal/tools/confirm.go` gates every write: `confirmWrite` sends an
`elicitation/create` request showing what will be written and waits for an
explicit `confirm: true`.

- Every write tool declares `change_summary` (via `withChangeSummary()`): the
  model's own explanation, in the user's language, shown verbatim in the prompt.
- **Fail-closed** — decline, cancel, missing `confirm`, a client that never
  advertised the `elicitation` capability, or a 10-minute timeout abort the write.
- The client capability is checked *before* sending: mcp-go does not check it
  and would block until the context expires.
- `TIMEPERFORMANCE_AUTO_WRITE=1` skips the flow entirely.
- CLI equivalent: every write command requires `--yes`; without it, it prints
  the change and sends nothing.

## API notes (v4)

- Base path is `/api/v4`; auth is HTTP Basic (OAuth 2.1 bearer also exists
  server-side but is not implemented here).
- `whoami` returns a bare JSON string, not an object: a sentence shaped like
  `Jane Doe (id: 4242) @ tenant`. `ResolveUserID("me")` parses the numeric
  id out of it (regexp `whoamiID` in client.go) — the name alone does not match
  a directory entry.
- Team management endpoints take **query parameters**, not a JSON body.
- `GET /projects/{id}/team` returns a wrapper, not a user: `{user, profile,
  costRate, rights}` where `rights` is the string `MEMBER`, `PROJECT_MANAGER`
  or null (no access). `CostRate` is a `UnitValue` plus `rateMode`.
- Some reference endpoints (`/obs`) answer 403 depending on account permissions,
  even with read credentials — a GET 403 means "not allowed to read this", not
  "you need write credentials".
- Leaves go through `POST /users/{id}/assignments/syncUnavailabilities`, a
  **whole sync** over `firstDay..lastDay`: anything not sent is deleted.
  `PlanUserLeave` (leave.go) reads `/assignments` first and merges, keeping
  every unavailability (archived types included). Leave types are the
  `/npactivities` items with `unavailable: true`.
- Deliverables are `type = "goal"` in the API; a deliverable's `iteration` is the
  phase it is planned for.
- Task `state` and WBS `state` enums are lowercase (`draft`, `ready`, `open`,
  `closed`, `cancelled`); progress modes are uppercase.
