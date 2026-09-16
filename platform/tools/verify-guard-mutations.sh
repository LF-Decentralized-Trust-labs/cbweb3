#!/usr/bin/env bash
# SPDX-License-Identifier: Apache-2.0
#
# Re-runs every mutation behind docs/guard-parity.md and reports, per guard, whether it
# actually failed. Nothing here is read from prose: each case breaks the subject, runs the
# named test, and reports the exit code.
#
# Three rules the harness enforces, each from a false "did not detect" in this audit:
#   - a mutation that does not apply aborts the case (never reported as a pass)
#   - a mutation that does not COMPILE is reported as INCONCLUSIVE, not as a detection
#   - every run uses `go test -count=1` (these guards read files outside their package,
#     which the Go test cache does not observe)
set -u
HARNESS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/guard-mutations" && pwd)"
cd "$(git rev-parse --show-toplevel)" || exit 1

pass=0; fail=0
declare -a REPORT
TREE_BEFORE="$(git status --porcelain)"

# apply <file> <python-literal-old> <python-literal-new>
apply() {
  python3 - "$1" "$2" "$3" <<'PY'
import io,sys
p,old,new = sys.argv[1],sys.argv[2],sys.argv[3]
s = io.open(p, encoding='utf-8').read()
if old not in s:
    sys.exit("MUTACAO-NAO-APLICADA")
io.open(p,'w',encoding='utf-8').write(s.replace(old,new,1))
PY
}

# case <label> <module-dir> <test-regex> <package> <file> <old> <new>
run_case() {
  local label="$1" mod="$2" re="$3" pkg="$4" file="$5" old="$6" new="$7"
  # Snapshot the file as it is NOW, not as HEAD has it. `git checkout -- <file>` was the obvious
  # restore and it is wrong: it discards uncommitted work in the file being mutated. It silently
  # destroyed a half-finished fix twice before this was written — once in a frontend store, once
  # in the very file whose guard was being verified.
  local snapshot; snapshot=$(mktemp)
  cp "$file" "$snapshot"
  restore() { cp "$snapshot" "$file"; rm -f "$snapshot"; }
  if ! apply "$file" "$old" "$new"; then
    REPORT+=("HARNESS  | $label | o padrão da mutação não casou — caso abortado")
    restore; fail=$((fail+1)); return
  fi
  local out rc
  out=$( (cd "$mod" && go test -count=1 "$pkg" -run "$re") 2>&1 ); rc=$?
  restore
  if grep -qE "build failed|cannot use|undefined:|too many arguments" <<<"$out"; then
    REPORT+=("INCONCL. | $label | a mutação não compila — não é detecção")
    fail=$((fail+1)); return
  fi
  if grep -q "no tests to run" <<<"$out" || ! grep -qE "^(ok|--- |FAIL|PASS)" <<<"$out"; then
    REPORT+=("HARNESS  | $label | o -run não casou com teste nenhum")
    fail=$((fail+1)); return
  fi
  if [ $rc -ne 0 ]; then
    REPORT+=("DETECTOU | $label | exit=$rc")
    pass=$((pass+1))
  else
    REPORT+=("PASSOU   | $label | exit=0 — A GUARDA NÃO GUARDA")
    fail=$((fail+1))
  fi
}

# new-file case: plant a file, run, remove
plant_case() {
  local label="$1" mod="$2" re="$3" pkg="$4" file="$5" body="$6"
  printf '%s' "$body" > "$file"
  local out rc
  out=$( (cd "$mod" && go test -count=1 "$pkg" -run "$re") 2>&1 ); rc=$?
  rm -f "$file"
  if [ $rc -ne 0 ]; then REPORT+=("DETECTOU | $label | exit=$rc"); pass=$((pass+1))
  else REPORT+=("PASSOU   | $label | exit=0 — A GUARDA NÃO GUARDA"); fail=$((fail+1)); fi
}

A=scenario-a/toolkit
B=scenario-b/toolkit

# ---------------------------------------------------------------- 1-2. teto de log
run_case "A  teto de log (template sem max-size)" "$A" \
  "TestProvisioningTemplates_CapContainerLogs" "./engine/orchestrator/..." \
  scenario-a/provisioning/templates/entity-backend/backend-compose.yaml \
  '    max-size: "50m"' '    max-size-REMOVIDO: "50m"'

run_case "B  teto de log (template sem max-size)" "$B" \
  "TestProvisioningTemplates_CapContainerLogs" "./engine/composetemplate/..." \
  scenario-b/provisioning/templates/entity-backend.compose.yaml \
  '    max-size: "50m"' '    max-size-REMOVIDO: "50m"'

# ---------------------------------------------------------- 3-4. portas efêmeras
run_case "A  porta efêmera (RPC dentro do range)" "$A" \
  "TestNoSampleHostPortReachesTheEphemeralRange" "./engine/orchestrator/..." \
  scenario-a/samples/argentina/bank-galicia.yaml \
  '      port: 8666' '      port: 40666'

run_case "B  porta efêmera (RPC dentro do range)" "$B" \
  "TestNoSampleHostPortReachesTheEphemeralRange" "./engine/orchestrator/..." \
  scenario-b/samples/argentina/bank-galicia.yaml \
  'rpc:  { port: 9246 }' 'rpc:  { port: 49246 }'

# ------------------------------------------------------------- 5-6. limite DNS 63
run_case "A  nome de container acima do rótulo DNS" "$A" \
  "TestGeneratedContainerNamesFitDNSLabel" "./engine/orchestrator/..." \
  scenario-a/samples/argentina/bank-galicia.yaml \
  '  name: bank-galicia' '  name: bank-galicia-saint-vincent-and-the-grenadines-extra'

run_case "B  nome de container acima do rótulo DNS" "$B" \
  "TestGeneratedContainerNamesFitDNSLabel" "./engine/apply/..." \
  scenario-b/samples/argentina/bank-galicia.yaml \
  '  name: bank-galicia' '  name: bank-galicia-saint-vincent-and-the-grenadines-extra'

# ------------------------------------------------------- 7-8. segredo em argv
run_case "B  segredo em argv (volta o --password)" "$B" \
  "TestKeycloakProvisioning_" "./engine/orchestrator/..." \
  scenario-b/toolkit/engine/orchestrator/keycloak_provision.go \
  '`KC_CLI_PASSWORD="$KC_BOOTSTRAP_ADMIN_PASSWORD" %s config credentials `' \
  '`%s config credentials --password $KC_ADMIN_PASSWORD `'

run_case "A  segredo em argv (volta o --password)" "$A" \
  "TestKeycloakReconcile_" "./engine/orchestrator/..." \
  scenario-a/toolkit/engine/orchestrator/keycloak_admin_users_reconcile.go \
  '`KC_CLI_PASSWORD="$KC_BOOTSTRAP_ADMIN_PASSWORD" %s config credentials `' \
  '`%s config credentials --password $KC_ADMIN_PASSWORD `'

# --------------------------------------------------- 9-10. literal de imagem
plant_case "A  literal de imagem plantado no pacote" "$A" \
  "TestNoImagePinLiteralsInThisPackage" "./engine/apply/..." \
  scenario-a/toolkit/engine/apply/__probe_pin.go \
  '// SPDX-License-Identifier: Apache-2.0

package apply

const probePin = "hyperledger/besu:25.8.0"
'

plant_case "B  literal de imagem plantado no pacote" "$B" \
  "TestNoImagePinLiteralsInThisPackage" "./engine/orchestrator/..." \
  scenario-b/toolkit/engine/orchestrator/__probe_pin.go \
  '// SPDX-License-Identifier: Apache-2.0

package orchestrator

const probePin = "hyperledger/besu:25.8.0"
'

# ------------------------------------------- 11. pré-requisitos do Keycloak (B)
run_case "B  join deixa de emitir sslRequired=NONE" "$B" \
  "TestKeycloakProvisioning_SetsWhatItsOwnAssertionsDemand" "./engine/orchestrator/..." \
  scenario-b/toolkit/engine/orchestrator/step_join.go \
  '	fmt.Fprintf(&b, "(%[1]s update realms/%[2]s -s sslRequired=NONE -s accessTokenLifespan=%[3]d || kcw '"'"'update realm settings'"'"') && ",
		kc, bankKeycloakRealm, accessTokenLifespanSeconds)
' \
  '	_ = accessTokenLifespanSeconds
'

# ------------------------------------------------- 12-13. DNS do proxy (A, novas)
run_case "A  upstream do proxy renomeado" "$A" \
  "TestProxyUpstreamsNameContainersTheTemplatesCreate" "./engine/orchestrator/..." \
  scenario-a/toolkit/engine/orchestrator/proxy.go \
  '{Segment: "governance", Upstream: prefix + "-governance-frontend:80"},' \
  '{Segment: "governance", Upstream: prefix + "-governance-spa:80"},'

run_case "A  upstream do proxy longo demais" "$A" \
  "TestProxyUpstreamsFitDNSLabel" "./engine/orchestrator/..." \
  scenario-a/toolkit/engine/orchestrator/proxy.go \
  '{Segment: "governance", Upstream: prefix + "-governance-frontend:80"},' \
  '{Segment: "governance", Upstream: prefix + "-central-bank-governance-portal-frontend:80"},'

# ------------------------------------------------ 14. recusas codificadas (B, vitest)
TRUST=scenario-b/frontend/apps/bank/src/services/api/trust-errors.ts
SNAP_TRUST=$(mktemp); cp "$TRUST" "$SNAP_TRUST"
if apply "$TRUST" 'const trustRejectionCodes: ReadonlySet<string> = new Set([
  RELAY_SIGNATURE_INVALID,
  RELAY_SIGNATURE_REQUIRED,' 'const trustRejectionCodes: ReadonlySet<string> = new Set([
  RELAY_SIGNATURE_INVALID,
]);
const unusedCodes = new Set([
  RELAY_SIGNATURE_REQUIRED,'; then
  out=$( (cd scenario-b/frontend/apps/bank && npx vitest run src/services/api/__tests__/trust-errors.test.ts) 2>&1 ); rc=$?
  cp "$SNAP_TRUST" "$TRUST"; rm -f "$SNAP_TRUST"
  if [ $rc -ne 0 ]; then REPORT+=("DETECTOU | B  recusas codificadas reduzidas a um código | exit=$rc"); pass=$((pass+1))
  else REPORT+=("PASSOU   | B  recusas codificadas reduzidas a um código | exit=0 — A GUARDA NÃO GUARDA"); fail=$((fail+1)); fi
else
  REPORT+=("HARNESS  | B  recusas codificadas | o padrão da mutação não casou"); fail=$((fail+1))
fi

# --------------------------------- 17-19. senha de OPERADOR fora do argv (A e B)
# A irmã do segredo de administrador, e a lacuna que estava nos DOIS cenários — o que uma
# comparação de paridade não acha por construção. Ver docs/scenario-drift.md §14.

run_case "A  volta o --new-password no set-password" "$A" \
  "TestKeycloakReconcile_" "./engine/orchestrator/..." \
  scenario-a/toolkit/engine/orchestrator/keycloak_admin_users_reconcile.go \
  'fmt.Fprintf(&b, "KC_CLI_PASSWORD=\"$%[4]s\" %[1]s set-password -r %[2]s --username %[3]s || true\n",
			kc, realm, u.Username, operatorPasswordVar(i))' \
  '_ = i
		fmt.Fprintf(&b, "%[1]s set-password -r %[2]s --username %[3]s --new-password %[4]s || true\n",
			kc, realm, u.Username, u.Password)'

run_case "B  volta o --new-password no set-password" "$B" \
  "OperatorPassword" "./engine/orchestrator/..." \
  scenario-b/toolkit/engine/orchestrator/step_found_spoke.go \
  'fmt.Fprintf(b, "(KC_CLI_PASSWORD=\"$%[4]s\" %[1]s set-password -r %[2]s --username %[3]s || kcw '"'"'set operator password'"'"')",
			kc, realm, u.Username, operatorPasswordVar(i))' \
  '_ = i
		fmt.Fprintf(b, "(%[1]s set-password -r %[2]s --username %[3]s --new-password %[4]s || kcw '"'"'set operator password'"'"')",
			kc, realm, u.Username, u.Password)'

# A senha sai do script mas entra no argv do docker: a forma `-e NOME=valor`, que é a que quase
# todo exemplo mostra. É a mesma exposição um passo à esquerda, e é o caso que um teste que só
# lê o script não veria.
for pair in "A|$A|scenario-a/toolkit/engine/orchestrator/keycloak_admin_users_reconcile.go" \
            "B|$B|scenario-b/toolkit/engine/orchestrator/keycloak_operator_password.go"; do
  IFS='|' read -r sc mod file <<<"$pair"
  run_case "$sc  docker recebe -e NOME=valor em vez de -e NOME" "$mod" \
    "TestDockerExecArgs" "./engine/orchestrator/..." "$file" \
    'args = append(args, "-e", name)' 'args = append(args, "-e", pair)'
done

# ------------------------------------- 15-16. segredo em argv, asserção de VALOR
# Os casos 7-8 acima exercitam só a asserção de FORMA: escrevem o texto literal
# "$KC_ADMIN_PASSWORD", que o shell expandiria mas que não põe o sentinela no script.
# Estes dois restauram o código anterior de verdade, com o valor resolvido interpolado —
# que é o que a asserção de valor existe para pegar.

KCB=scenario-b/toolkit/engine/orchestrator/keycloak_provision.go
SNAP_KCB=$(mktemp); cp "$KCB" "$SNAP_KCB"
if python3 "$HARNESS_DIR/mut_b_value.py" "$KCB"; then
  out=$( (cd "$B" && go test -count=1 ./engine/orchestrator/... -run "TestKeycloakProvisioning_NeverEmbedsTheAdminSecret") 2>&1 ); rc=$?
  cp "$SNAP_KCB" "$KCB"; rm -f "$SNAP_KCB"
  if grep -q "build failed" <<<"$out"; then REPORT+=("INCONCL. | B  segredo em argv, VALOR | a mutação não compila"); fail=$((fail+1))
  elif [ $rc -ne 0 ]; then REPORT+=("DETECTOU | B  segredo em argv, asserção de VALOR | exit=$rc"); pass=$((pass+1))
  else REPORT+=("PASSOU   | B  segredo em argv, asserção de VALOR | exit=0 — A GUARDA NÃO GUARDA"); fail=$((fail+1)); fi
else
  REPORT+=("HARNESS  | B  segredo em argv, VALOR | padrão não casou"); fail=$((fail+1))
fi

KCA=scenario-a/toolkit/engine/orchestrator/keycloak_admin_users_reconcile.go
RTA=scenario-a/toolkit/engine/orchestrator/keycloak_admin_users_reconcile_test.go
SNAP_KCA=$(mktemp); cp "$KCA" "$SNAP_KCA"
SNAP_RTA=$(mktemp); cp "$RTA" "$SNAP_RTA"
if python3 "$HARNESS_DIR/mut_a_value.py" "$KCA" "$RTA"; then
  out=$( (cd "$A" && go test -count=1 ./engine/orchestrator/... -run "TestKeycloakReconcile_NeverEmbedsTheAdminSecret") 2>&1 ); rc=$?
  cp "$SNAP_KCA" "$KCA"; cp "$SNAP_RTA" "$RTA"; rm -f "$SNAP_KCA" "$SNAP_RTA"
  if grep -q "build failed" <<<"$out"; then REPORT+=("INCONCL. | A  segredo em argv, VALOR | a mutação não compila"); fail=$((fail+1))
  elif [ $rc -ne 0 ]; then REPORT+=("DETECTOU | A  segredo em argv, asserção de VALOR | exit=$rc"); pass=$((pass+1))
  else REPORT+=("PASSOU   | A  segredo em argv, asserção de VALOR | exit=0 — A GUARDA NÃO GUARDA"); fail=$((fail+1)); fi
else
  REPORT+=("HARNESS  | A  segredo em argv, VALOR | padrão não casou"); fail=$((fail+1))
fi

# ------------------------------- 17-23. convergência do realm Keycloak (A, novas)
# As guardas da convergência de realm: quatro do PR #236 (readiness, redirectUris, ordem
# canônica, comparação por conjunto) e três das correções pedidas na review (criar o realm
# que o import ignora, não chamar kcadm antes do servidor subir, convergir no join). A quinta
# do PR — segredo em argv — já está nos casos 8 e 16.
#
# Registradas aqui porque o harness é o que torna a afirmação "estas guardas guardam"
# reproduzível por quem revisa, em vez de uma tabela no corpo do PR.
#
# ATENÇÃO: run_case restaura com `git checkout --`, então o harness só verifica código
# COMMITADO — rodá-lo sobre edições pendentes nos arquivos que ele muta as descarta.

KCSTEP=scenario-a/toolkit/engine/orchestrator/step_provision_keycloak.go
KCREC=scenario-a/toolkit/engine/orchestrator/keycloak_realm_reconcile.go
KCORD=scenario-a/toolkit/engine/orchestrator/step.go

# A gate volta para /realms/master — o realm que existe em todo Keycloak.
run_case "A  readiness do Keycloak volta a olhar master" "$A" \
  "TestProvisionKeycloak_UnimportedRealmIsNotSatisfied" "./engine/orchestrator/..." \
  "$KCSTEP" \
  '	for _, plan := range s.realms {
		if !probe(ctx, s.realmURL(plan.Realm)) {' \
  '	for _, plan := range s.realms {
		_ = plan
		if !probe(ctx, s.realmURL("master")) {'

# O Run volta a só esperar: sem criar o realm que o import ignorou.
run_case "A  Run deixa de criar o realm que o import ignora" "$A" \
  "TestProvisionKeycloak_CreatesTheRealmTheImportSkipped" "./engine/orchestrator/..." \
  "$KCSTEP" \
  '		if !created && !time.Now().Before(graceOver) && s.serverUp(ctx) {' \
  '		if false && !created && !time.Now().Before(graceOver) && s.serverUp(ctx) {'

# kcadm chamado contra um container que ainda não responde.
run_case "A  create do realm sem esperar o servidor subir" "$A" \
  "TestProvisionKeycloak_DoesNotReachForKcadmBeforeTheServerAnswers" "./engine/orchestrator/..." \
  "$KCSTEP" \
  '		if !created && !time.Now().Before(graceOver) && s.serverUp(ctx) {' \
  '		if !created && !time.Now().Before(graceOver) {'

# Comparação de origens como SUBCONJUNTO de verdade (want ⊆ have, extras tolerados) —
# a mutação que o PR registrou como tendo passado por estar errada na primeira tentativa.
run_case "A  origens comparadas como subconjunto" "$A" \
  "TestReconcileKeycloakRealm_UndeclaredOriginIsNotSatisfied" "./engine/orchestrator/..." \
  "$KCREC" \
  '	if len(have) != len(want) {
		return false
	}
	h := append([]string(nil), have...)
	w := append([]string(nil), want...)
	sort.Strings(h)
	sort.Strings(w)
	for i := range h {
		if h[i] != w[i] {
			return false
		}
	}
	return true' \
  '	_ = sort.Strings
	for _, wanted := range want {
		found := false
		for _, held := range have {
			if held == wanted {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true'

# O reconcile deixa de escrever redirectUris: o Check continua exigindo, o Run não entrega.
run_case "A  reconcile para de escrever redirectUris" "$A" \
  "TestReconcileKeycloakRealm_RunSetsEverythingItChecks" "./engine/orchestrator/..." \
  "$KCREC" \
  '		fmt.Fprintf(&b, "%[1]s update clients/$KCID -r %[2]s -s '"'"'webOrigins=%[3]s'"'"' -s '"'"'redirectUris=%[4]s'"'"' || exit 1\n",
			kc, plan.Realm, origins, redirects)' \
  '		_ = redirects
		fmt.Fprintf(&b, "%[1]s update clients/$KCID -r %[2]s -s '"'"'webOrigins=%[3]s'"'"' || exit 1\n",
			kc, plan.Realm, origins)'

# O passo sai da ordem canônica do found: nunca roda, e nenhum outro teste percebe.
run_case "A  reconcile do realm fora da ordem do found" "$A" \
  "TestReconcileKeycloakRealmIsInTheCanonicalFoundOrder" "./engine/orchestrator/..." \
  "$KCORD" \
  '	StepReconcileKeycloakRealm,
	StepStartCBBackend,' \
  '	StepStartCBBackend,'

# A convergência sai do join: volta a valer só para banco central.
run_case "A  join volta a não convergir nada" "$A" \
  "TestKeycloakConvergenceReachesCommercialBanks" "./engine/orchestrator/..." \
  "$KCORD" \
  '	StepReconcileAdminUsers,
	StepReconcileKeycloakRealm,
	StepStartBackend,' \
  '	StepStartBackend,'

echo
echo "================ VERIFICAÇÃO POR MUTAÇÃO ================"
printf '%s\n' "${REPORT[@]}"
echo "========================================================="
echo "detectaram: $pass   não detectaram/inconclusivos: $fail"
# Compared against the tree as it was when the run started, not against a clean tree:
# the harness must leave no residue, but the caller's own pending edits are none of its
# business and reporting them as residue would train the reader to ignore this line.
if [ "$(git status --porcelain)" = "$TREE_BEFORE" ]; then
  echo "árvore restaurada (nenhum resíduo do harness)"
else
  echo "AVISO: o harness deixou resíduo — diferença face ao estado inicial:"
  diff <(printf '%s\n' "$TREE_BEFORE") <(git status --porcelain) | sed 's/^/  /'
  fail=$((fail+1))
fi
[ "$fail" -eq 0 ]
