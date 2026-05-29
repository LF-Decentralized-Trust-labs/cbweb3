# Frontend Apps Monorepo Guide

Each app must live under `frontend/apps/<app-name>` and consume shared packages from `frontend/packages`.

## New app checklist

1. Create the app with Vite React TS:
   - `npm create vite@latest apps/<app-name> -- --template react-ts`
2. Add workspace deps in `apps/<app-name>/package.json`:
   - `"@cbweb3/ui": "*"`
   - `"@cbweb3/config": "*"` (devDependency)
3. Add Tailwind + PostCSS configs by copying from `apps/bank`:
   - `tailwind.config.cjs`
   - `postcss.config.cjs`
4. Import shared UI stylesheet in `src/main.tsx`:
   - `import "@cbweb3/ui/styles.css"`
5. Set TypeScript shared config:
   - `tsconfig.app.json` extends `../../packages/config/tsconfig.base.json`
   - `tsconfig.node.json` extends `../../packages/config/tsconfig.base.json`
   - add TS alias for shared UI shadcn imports:
     - `"baseUrl": "."`
     - `"paths": { "@/*": ["../../packages/ui/src/*"] }`
6. Add Vite alias in `vite.config.ts`:
   - `"@": path.resolve(__dirname, "../../packages/ui/src")`
7. Use shared ESLint config:
   - `eslint.config.js` exports `@cbweb3/config/eslint/react`
8. Use UI components from package imports only:
   - `import { Button } from "@cbweb3/ui"`

## Important rule

Run shadcn component generation only in `frontend/packages/ui`, so every app reuses a single UI source of truth.
