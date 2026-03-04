#!/usr/bin/env bash
set -euo pipefail

APP_NAME="${1:-}"

if [[ -z "$APP_NAME" ]]; then
  echo "Usage: ./create-frontend-app.sh <app-name>"
  exit 1
fi

if [[ ! "$APP_NAME" =~ ^[a-z0-9-]+$ ]]; then
  echo "Error: app-name must use lowercase letters, numbers, and hyphens only."
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FRONTEND_DIR="$SCRIPT_DIR"
APP_DIR="$FRONTEND_DIR/apps/$APP_NAME"

if [[ -d "$APP_DIR" ]]; then
  echo "Error: app already exists at $APP_DIR"
  exit 1
fi

cd "$FRONTEND_DIR"

echo "Scaffolding Vite React TypeScript app: apps/$APP_NAME"
npm create vite@latest "apps/$APP_NAME" -- --template react-swc-ts --no-interactive

cd "$APP_DIR"

npm pkg set "dependencies.@cbweb3/ui=*"
npm pkg set "devDependencies.@cbweb3/config=*"
npm pkg set "devDependencies.@tailwindcss/postcss=^4.1.12"
npm pkg set "devDependencies.tailwindcss=^4.1.12"
npm pkg set "devDependencies.postcss=^8.5.6"
npm pkg set "devDependencies.autoprefixer=^10.4.21"

cat > eslint.config.js <<'EOF'
import reactConfig from '@cbweb3/config/eslint/react'

export default reactConfig
EOF

cat > tsconfig.app.json <<'EOF'
{
  "extends": "../../packages/config/tsconfig.base.json",
  "compilerOptions": {
    "tsBuildInfoFile": "./node_modules/.tmp/tsconfig.app.tsbuildinfo",
    "baseUrl": ".",
    "useDefineForClassFields": true,
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "paths": {
      "@/*": ["../../packages/ui/src/*"]
    },
    "types": ["vite/client"],
    "noEmit": true,
    "erasableSyntaxOnly": true
  },
  "include": ["src"]
}
EOF

cat > tsconfig.node.json <<'EOF'
{
  "extends": "../../packages/config/tsconfig.base.json",
  "compilerOptions": {
    "tsBuildInfoFile": "./node_modules/.tmp/tsconfig.node.tsbuildinfo",
    "target": "ES2023",
    "lib": ["ES2023"],
    "types": ["node"],
    "noEmit": true,
    "erasableSyntaxOnly": true,
    "module": "ESNext"
  },
  "include": ["vite.config.ts"]
}
EOF

cat > vite.config.ts <<'EOF'
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc";
import path from "path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "../../packages/ui/src"),
    },
  },
});
EOF

cat > postcss.config.cjs <<'EOF'
module.exports = {
  plugins: {
    "@tailwindcss/postcss": {},
    autoprefixer: {},
  },
};
EOF

cat > src/main.tsx <<'EOF'
import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import '@cbweb3/ui/styles.css'
import App from './App.tsx'

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
EOF

cat > src/App.tsx <<EOF
import { Button } from '@cbweb3/ui'

function App() {
  return (
    <main className="mx-auto flex min-h-screen max-w-4xl flex-col items-center justify-center gap-6 p-6">
      <h1 className="text-3xl font-semibold tracking-tight">CBWeb3 ${APP_NAME^} App</h1>
      <p className="text-muted-foreground">Shared shadcn/ui components from @cbweb3/ui</p>
      <div className="flex items-center gap-3">
        <Button>Primary Action</Button>
        <Button variant="outline">Secondary Action</Button>
      </div>
    </main>
  )
}

export default App
EOF

rm -f src/App.css src/index.css src/assets/react.svg
rmdir src/assets 2>/dev/null || true

cd "$FRONTEND_DIR"
npm pkg set "scripts.dev:$APP_NAME=npm run dev --workspace=$APP_NAME --"
npm install

echo ""
echo "Done. App created at apps/$APP_NAME"
echo "Run: npm run dev:$APP_NAME"
