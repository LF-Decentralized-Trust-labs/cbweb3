#!/usr/bin/env bash
set -euo pipefail

# cbweb3-platform skeleton initializer
# Usage: bash init_cbweb3-platform.sh

mkdir -p contracts/{src,test,scripts} \
         backend/{services/{payments,fx,ledger-gateway,compliance,identity},shared,config} \
         frontend/{apps/console,packages/ui,public,src} \
         apis/{openapi,sdk/{ts,python,java}} \
         interop/{hub-and-spoke/{cacti,ccip},single-ledger} \
         deploy/{terraform,helm-k8s,cloud-run} \
         tests/{unit,integration,e2e,performance} \
         docs/{architecture,design,governance,runbooks} \
         .github/workflows

# Placeholders so Git tracks dirs
find contracts backend frontend apis interop deploy tests docs -type d -empty -print0 | xargs -0 -I {} sh -c 'touch "{}/.gitkeep"'

# .gitignore baseline
cat > .gitignore <<'EOF'
# Node / Frontend
node_modules/
dist/
.next/
coverage/
*.log

# Python / Go caches
__pycache__/
*.pyc

# Environment
.env
.env.*

# Terraform
.terraform/
terraform.tfstate*
*.tfvars

# Generated SDKs / Artifacts
apis/sdk/*/dist/
build/
artifacts/

# OS
.DS_Store
EOF

# Minimal CI placeholder
cat > .github/workflows/ci.yml <<'YAML'
name: ci
on:
  push: { branches: [main] }
  pull_request: { branches: [main] }
jobs:
  show-structure:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Show repository tree
        run: ls -R
YAML

echo "Skeleton created. Next steps:"
echo "  1) Add README.md at repository root."
echo "  2) git add . && git commit -m 'chore(platform): initial skeleton'"
echo "  3) git push -u origin main"
