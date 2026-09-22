# Agent instructions

## Project

`radar-obs` contains the Radar Agent and Radar Gateway observability runtime.

## Agent skills

### Issue tracker

Work is tracked as local markdown issues under `.scratch/`; use the conventions in `docs/agents/issue-tracker.md`.

### Domain docs

This is a single-context repository. Read `CONTEXT.md` and relevant records under `docs/adr/` when they exist. See `docs/agents/domain.md`.

## Engineering expectations

- Prefer OpenTelemetry Collector Contrib components over reimplementing OTLP receivers, processors, exporters, or ClickHouse persistence.
- Keep local development runnable without Docker when practical.
- Validate changes with focused tests and diagnostics before broader validation.
- Never commit credentials from `.env` files.
