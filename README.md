<div align="center">

# timeperformance-mcp

**Your project portfolio, one prompt away.**

A single Go binary that puts the [TimePerformance](https://timeperformance.com) API v4
in front of any MCP client, and in your shell.

[![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![MCP](https://img.shields.io/badge/MCP-stdio-8A63D2)](https://modelcontextprotocol.io)
[![API](https://img.shields.io/badge/TimePerformance-API%20v4-1f8ceb)](https://pma.timeperformance.com/apidoc/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Stars](https://img.shields.io/github/stars/edouard-claude/timeperformance-mcp?style=flat&logo=github&color=f5c518)](https://github.com/edouard-claude/timeperformance-mcp/stargazers)

</div>

---

Ask your assistant *"what am I planned on next month?"*, *"who is on the Vestia project?"*,
*"how many hours went into Lot 2 in August?"* and get an answer from the real data.
Same binary, same operations, from a terminal:

```console
$ tp-mcp assignments me --from 2026-09-01 --to 2026-09-30
# Assignments for user #4242 (44 half-days)

- 2026-09-01 AM -> Pilotask (#1288046)
- 2026-09-01 PM -> Pilotask (#1288046)
- 2026-09-02 AM -> Zenenco (#1239135)
...
```

## Why

TimePerformance holds the schedule, the WBS, the timesheets and the earned-value
indicators of a whole portfolio. This server exposes **37 tools** over it: 22 reads
and 15 writes, each write gated by a human confirmation before anything is sent.

- **Two interfaces, one binary.** MCP stdio server by default, CLI subcommands
  otherwise. Scripts and agents run the same code path.
- **Names, not ids.** `project` takes an id or an exact name, `user` takes an id,
  a full name, an email, or `me`. Resolution happens server-side.
- **Fail-closed writes.** Nothing reaches the API without an explicit human yes.
- **Rate-limit aware.** The API allows one request per second and one in flight;
  the client serializes and paces calls, and honours `Retry-After` on a 429.

## Install

```bash
git clone https://github.com/edouard-claude/timeperformance-mcp.git
cd timeperformance-mcp
make install   # -> /usr/local/bin/tp-mcp   (make build for a local ./tp-mcp)
```

## Credentials

The API uses HTTP Basic with **dedicated API credentials**, never your login
email and password.

| Credential | Where to get it | Access |
|---|---|---|
| **User** | Web app, profile, Settings | read-only (`GET`), scoped to your own permissions |
| **Back-office** | Web app, Administration, API BackOffice | full read/write, exempt from the per-second limit |

```bash
export TIMEPERFORMANCE_LOGIN=<key-id>@<tenant>
export TIMEPERFORMANCE_PASSWORD=<secret>
export TIMEPERFORMANCE_URL=https://pma.timeperformance.com   # optional
```

`TP_LOGIN`, `TP_PASSWORD` and `TP_URL` work as short aliases. With user
credentials every write returns `HTTP 403`: that is the API's design, and the
error message says so.

Check it works:

```console
$ tp-mcp whoami
Authenticated as: Jane Doe (id: 4242) @ tenant
Instance: https://pma.timeperformance.com
```

## Wire it to an MCP client

<details open>
<summary><b>Claude Code</b></summary>

```bash
claude mcp add --scope user timeperformance \
  --env TIMEPERFORMANCE_LOGIN=<key-id>@<tenant> \
  --env TIMEPERFORMANCE_PASSWORD=<secret> \
  -- /usr/local/bin/tp-mcp
```
</details>

<details>
<summary><b>Claude Desktop, or any MCP client</b></summary>

```json
{
  "mcpServers": {
    "timeperformance": {
      "command": "/usr/local/bin/tp-mcp",
      "env": {
        "TIMEPERFORMANCE_LOGIN": "<key-id>@<tenant>",
        "TIMEPERFORMANCE_PASSWORD": "<secret>"
      }
    }
  }
}
```
</details>

## Tools

### Read

| Tool | What you get |
|---|---|
| `whoami` | The identity behind the configured credentials |
| `list_projects` | Projects, filtered by name, archived or templates |
| `get_project` | State, client, type, priority, labels, description |
| `get_project_report` | `progress` (earned value, RAG, cost and effort), `roadmap`, `brief`, `baseline`, `config`, `workload`, `actuals`, `assignments`, `datasheets`, `lastmodified`, `risks` |
| `list_project_team` | Members, project-manager right, skill profile, cost rate |
| `list_project_tasks`, `get_task` | Tasks with performer, state, effort spent and remaining, check-list |
| `list_project_deliverables`, `get_deliverable` | Deliverables tree, milestones, planned dates, acceptance criteria |
| `list_project_phases`, `get_phase` | Phases tree and sub-phases |
| `list_project_expenses`, `get_expense` | Expenses with amount, date, counterparty |
| `list_users`, `get_user` | Directory, one account with its application rights |
| `get_user_timesheet` | Hours per project, per task and per day |
| `get_user_time_report` | Hours per project and per non-project activity |
| `list_user_tasks` | Tasks of a user, or their to-do list with `scope=todo` |
| `get_user_assignments` | Half-day schedule: what someone is planned on |
| `list_portfolios`, `get_portfolio_progress_report` | Portfolios and their consolidated progress |
| `list_reference_data` | `obs`, `profiles`, `npactivities`, `projectstates`, `customforms` |

### Write

| Tool | What it does |
|---|---|
| `create_project`, `update_project` | Create (optionally from a template), patch fields |
| `create_task`, `update_task` | Create and update tasks |
| `create_deliverable`, `update_deliverable` | Deliverables and sub-deliverables |
| `create_phase`, `update_phase` | Phases and sub-phases |
| `create_expense`, `update_expense` | Project expenses |
| `create_user`, `update_user`, `archive_user` | Accounts and application rights |
| `manage_project_member` | Add, promote or remove a team member |
| `delete_element` | Permanently delete a task, deliverable, phase, expense or datasheet |

Reports come back as the API's own JSON: read the indicators you need instead of
trusting a hand-written mapping of forty report schemas.

## Nothing gets written without a human

Every write tool sends an MCP `elicitation/create` request showing exactly what
will be sent, and waits.

```
Update a TimePerformance task

Move the Vestia integration task to Marine, it is her sprint now.

Task: #4213
Performer (user id): #1161863
Due date: 2026-09-30

Apply this to TimePerformance?
```

- Each write tool requires a `change_summary`: the model explains, in your own
  language, what it is about to change and why. That sentence is shown verbatim.
- **Fail-closed.** A decline, a cancel, a missing confirmation, a client that never
  advertised the `elicitation` capability, or ten minutes of silence all abort the write.
- `TIMEPERFORMANCE_AUTO_WRITE=1` skips the flow for clients that already prompt
  before every tool call.

The CLI has no elicitation, so a write command is a dry run until you pass `--yes`:

```console
$ tp-mcp update-task 4213 --performer "Marine Sanhard" --due-date 2026-09-30
Update task #4213:

Performer: #1161863
Due date: 2026-09-30

Nothing was sent. Re-run with --yes to apply.
```

## CLI cheat sheet

```bash
tp-mcp list-projects --name vestia            # find a project
tp-mcp get-project "SQ - Le14 Vestia"         # by name or id
tp-mcp list-tasks 1280971 --not-closed        # open tasks
tp-mcp list-deliverables 1280971              # WBS tree
tp-mcp list-team 1280971                      # who is on it
tp-mcp timesheet me --from 2026-08-01 --to 2026-08-31
tp-mcp user-tasks me --scope todo             # current to-do list
tp-mcp project-report 1280971 --kind progress --with-deliverables
tp-mcp reference --kind profiles              # ids the write tools expect
tp-mcp help                                   # everything else
```

Flags and positional arguments can be given in any order.

## Layout

```
cmd/tp-mcp/            entry point: MCP stdio server, or CLI dispatch
internal/
├── timeperformance/   REST client, API methods, types
├── tools/             MCP tool registrations, confirmation flow, formatters
└── cli/               CLI commands (reads and writes)
```

Built against [the official OpenAPI definition](https://pma.timeperformance.com/apidoc/api/openapi.json).
Not affiliated with TimePerformance.

## License

MIT, see [LICENSE](LICENSE).
