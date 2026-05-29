# CBWeb3 Frontend Workspace

This folder contains the frontend monorepo, managed with npm workspaces.

## Workspace Structure

```text
frontend/
├─ apps/
│  ├─ bank/          # Main bank portal
│  └─ supervisor/    # Supervisor portal
├─ scripts/
│  └─ create-frontend-app.sh
├─ packages/
│  ├─ ui/            # Shared UI components + global styles
│  └─ config/        # Shared TS/ESLint/Tailwind preset config
```

## Stack

- React 19 + TypeScript
- Vite 7
- Tailwind CSS v4
- Shared UI package: `@cbweb3/ui`

## Prerequisites

- Node.js `22.13+` (recommended)
- npm `10+`

Check versions:

```bash
node -v
npm -v
```

## Local Setup

From the `frontend` folder:

```bash
npm install
```

### Run apps in development

Run one app:

```bash
npm run dev:bank
# or
npm run dev:supervisor
```

Run both apps (recommended): use two terminals

Terminal 1:

```bash
cd frontend
npm run dev:bank
```

Terminal 2:

```bash
cd frontend
npm run dev:supervisor
```

> Note: in this repository, `npm run dev` should **not** be relied on to keep both dev servers running concurrently.

## Build, Lint, Type-check

Build all workspaces:

```bash
npm run build
```

Build a single app:

```bash
npm run build --workspace=bank
npm run build --workspace=supervisor
```

Lint all workspaces:

```bash
npm run lint
```

Type-check a specific app:

```bash
npm run type-check --workspace=bank
```

## Docker Spoke Stacks

From the repository root, you can bring up frontend stacks with `make`:

```bash
make frontend-spoke-a
make frontend-spoke-b
make frontend-spoke-all
make frontend-scenario-b
```

Available variants:

```bash
make frontend-spoke-a-down
make frontend-spoke-b-down
make frontend-spoke-all-down
make frontend-scenario-b-down

make frontend-spoke-a-logs
make frontend-spoke-b-logs
make frontend-spoke-all-logs
make frontend-scenario-b-logs
```

Stack composition:

- `spoke-a`: Bank A, Bank C, Central Bank A
- `spoke-b`: Bank B, Bank D, Central Bank B
- `spoke-all`: Bank A, Bank B, Bank C, Bank D, Central Bank A, Central Bank B
- `scenario-b`: Bank A, Bank B, Bank C, Bank D, Central Bank A, Central Bank B with `VITE_SCENARIO=b` for all containers

## Portal Ports

Default exposed frontend ports (from `frontend/.env` and `frontend/.env.example`):

| Portal | App | Local URL | Backend API |
| --- | --- | --- | --- |
| Bank A | `bank` | http://localhost:5173 | http://localhost:18080/api/v1/ |
| Bank B | `bank` | http://localhost:5174 | http://localhost:28080/api/v1/ |
| Bank C | `bank` | http://localhost:5175 | http://localhost:48080/api/v1/ |
| Bank D | `bank` | http://localhost:5176 | http://localhost:58080/api/v1/ |
| Central Bank A | `governance` | http://localhost:5177 | http://localhost:38080/api/v1/ |
| Central Bank B | `governance` | http://localhost:5178 | http://localhost:60080/api/v1/ |

Notes:

- These ports are used by the Docker spoke stacks (`make frontend-spoke-*`).
- You can change any portal port by editing `frontend/.env` before running the stack.
- Local Vite dev servers do not hardcode a fixed port in app config; Vite picks an available one unless you pass `--port`.

Example for fixed local dev ports:

```bash
npm run dev:bank -- --port 5173
npm run dev:governance -- --port 5177
```

## Tailwind v4 Setup Notes

- PostCSS plugin is `@tailwindcss/postcss`.
- Shared global stylesheet lives in `packages/ui/src/styles.css`.
- Tailwind class detection is defined with `@source` directives in that file.
- New apps scaffolded with `scripts/create-frontend-app.sh` are generated with Tailwind v4-compatible setup.

## Creating a New Frontend App

From `frontend`:

```bash
./scripts/create-frontend-app.sh my-new-app
```

Then run:

```bash
npm run dev:my-new-app
```

The scaffold script sets up:

- Vite React + TypeScript app
- workspace deps on `@cbweb3/ui` and `@cbweb3/config`
- Tailwind v4-compatible PostCSS config
- shared alias wiring for UI source imports

## Troubleshooting

### Styles look broken or missing

1. Restart dev server(s) after CSS/config changes.
2. Ensure dependencies are installed from `frontend`:

```bash
cd frontend
npm install
```

3. Confirm each app imports shared styles in `src/main.tsx`:

```ts
import "@cbweb3/ui/styles.css"
```

### Workspace command fails

Run commands from `frontend`, not repository root.

### Clean reinstall

```bash
cd frontend
rm -rf node_modules package-lock.json
npm install
```

## Useful Paths

- `apps/bank/src/main.tsx`
- `apps/supervisor/src/main.tsx`
- `packages/ui/src/index.ts`
- `packages/ui/src/styles.css`
- `scripts/create-frontend-app.sh`
