# dispatcher

> [scenario-a](../../../README.md) › [frontend](../../README.md) › dispatcher

The **dispatcher** is a minimal, form-based utility app for manually triggering platform events and coordinating settlement steps. It serves as a developer tool and operations aid — useful for testing flows, triggering manual settlement steps, and debugging without going through the full bank portal UI.

---

## Architecture Placement

```
Browser (Developer / Ops)
         │
         ▼
  [dispatcher]
         │
         ▼
  [api-gateway]   REST (any entity)
```

---

## Users

Developers, QA engineers, and operations staff who need to trigger specific payment or settlement events directly.

---

## Key Features

- **Event dispatch forms** — Simple forms to submit specific API calls: lock HTLC, reveal secret, trigger FX agreement steps.
- **Manual settlement coordination** — Step through the cross-spoke HTLC flow manually for debugging or demo purposes.
- **Response viewer** — Displays raw API responses for inspection.

---

## Key Details

| Property | Value |
|----------|-------|
| Framework | React 19 + TypeScript |
| Build tool | Vite 7 |
| Styling | Tailwind CSS v4 + `@cbweb3/ui` |
| API | Axios → api-gateway REST |

### Dev

```bash
cd frontend
npm run dev:dispatcher
```

---

## Related

- [frontend workspace](../../README.md) — monorepo setup
- [bank portal](../bank/README.md) — full-featured alternative for normal operator workflows
- [tryouts](../../../tryouts/) — shell script equivalents of the manual flows this tool covers
