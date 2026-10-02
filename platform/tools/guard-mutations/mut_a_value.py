# SPDX-License-Identifier: Apache-2.0

import io, sys
src, test = sys.argv[1], sys.argv[2]
s = io.open(src, encoding='utf-8').read()
subs = [
    ('func adminUsersReadScript(kc, realm string, users []KeycloakUserPlan) string {\n'
     '\tvar b strings.Builder\n'
     '\tfmt.Fprintf(&b, "%s >/dev/null 2>&1 || exit 1\\n", kcadmLogin(kc))',
     'func adminUsersReadScript(kc, realm, adminPassword string, users []KeycloakUserPlan) string {\n'
     '\tvar b strings.Builder\n'
     '\tfmt.Fprintf(&b, "%[1]s config credentials --user admin --password %[2]s >/dev/null 2>&1 || exit 1\\n", kc, adminPassword)'),
    ('func adminUsersReconcileScript(kc, realm string, users []KeycloakUserPlan) string {\n'
     '\tvar b strings.Builder\n'
     '\tfmt.Fprintf(&b, "%s || exit 1\\n", kcadmLogin(kc))',
     'func adminUsersReconcileScript(kc, realm, adminPassword string, users []KeycloakUserPlan) string {\n'
     '\tvar b strings.Builder\n'
     '\tfmt.Fprintf(&b, "%[1]s config credentials --user admin --password %[2]s || exit 1\\n", kc, adminPassword)'),
    ('adminUsersReadScript(keycloakAdminCLI, realm, users)',
     'adminUsersReadScript(keycloakAdminCLI, realm, s.kcAdminPass, users)'),
    ('adminUsersReconcileScript(keycloakAdminCLI, realm, users)',
     'adminUsersReconcileScript(keycloakAdminCLI, realm, s.kcAdminPass, users)'),
]
for old, new in subs:
    if old not in s:
        sys.exit("MUTACAO-NAO-APLICADA: " + old[:60])
    s = s.replace(old, new, 1)
io.open(src, 'w', encoding='utf-8').write(s)

# the existing reconcile test calls the post-fix signature; move it back too, or the
# package does not compile and a build failure would be misread as a detection.
t = io.open(test, encoding='utf-8').read()
old_call = 'adminUsersReconcileScript(keycloakAdminCLI, "cb-realm",'
if old_call not in t:
    sys.exit("MUTACAO-NAO-APLICADA: chamada no teste")
io.open(test, 'w', encoding='utf-8').write(
    t.replace(old_call, 'adminUsersReconcileScript(keycloakAdminCLI, "cb-realm", "pw",', 1))