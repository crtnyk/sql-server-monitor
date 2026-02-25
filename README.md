# SQL Server Monitor

SQL Server performance monitor - collects DMV metrics, detects issues via severity scoring, stores locally as CSV, syncs to S3-compatible storage. Single Go binary, scheduled externally.

## Why

Built to automate SQL Server performance diagnostics. Identifies what's actually causing slowness - whether it's resource pressure, blocking, inefficient stored procedures, or wait accumulation - so troubleshooting is evidence-based instead of guesswork.

## Metrics Collected

- **Stored procedure stats** - execution counts, CPU time, logical reads, elapsed time
- **Active queries** - session info, wait types, blocking status, query text
- **Server resources** - CPU/memory, page life expectancy, buffer cache hit ratio, disk latency
- **Blocking chains** - recursive blocking detection with wait times and query context
- **Wait statistics** - filtered wait types with resource vs signal wait breakdown

## Severity Scoring

**GREEN** → normal | **YELLOW** → warning (CPU >80%, wait >30s, PLE <300, blocking, high reads) | **RED** → critical (wait >120s)

Detailed captures (blocking + waits) triggered automatically on YELLOW/RED.

## Setup

1. Copy `.env.example` to `.env` and fill in your SQL Server and S3 credentials
2. `make build` (or `make build-windows` for Windows)
3. Schedule via Task Scheduler or cron - runs once per invocation, collects → scores → stores → syncs