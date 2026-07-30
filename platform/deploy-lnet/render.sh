#!/usr/bin/env bash
# Render every *.yaml.tmpl under deploy-lnet into a sibling *.yaml, substituting the
# ${IP_*} address markers from addresses.env (or the current environment).
#
# Usage:
#   ./render.sh                 # render all templates
#   IP_HUB=1.2.3.4 ./render.sh  # override one address ad hoc
#
# Rendered *.yaml files are generated artifacts (git-ignored) — never edit them by hand.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=/dev/null
source "$HERE/addresses.env"

# Restrict substitution to our known vars so nothing else in the YAML is touched.
VARS='${IP_HUB} ${IP_CB_BRAZIL} ${IP_CB1} ${IP_CB2} ${IP_CB_COLOMBIA} ${IP_CB3} ${IP_CB4}'

count=0
while IFS= read -r tmpl; do
  out="${tmpl%.tmpl}"
  envsubst "$VARS" < "$tmpl" > "$out"
  # Fail loudly if any marker was left unresolved (e.g. a typo'd/new ${IP_*}).
  if grep -q '\${IP_' "$out"; then
    echo "ERROR: unresolved \${IP_*} marker in $out — check addresses.env" >&2
    exit 1
  fi
  echo "rendered ${out#"$HERE"/}"
  count=$((count + 1))
done < <(find "$HERE" -name '*.yaml.tmpl' | sort)

echo "rendered $count manifest(s)."
