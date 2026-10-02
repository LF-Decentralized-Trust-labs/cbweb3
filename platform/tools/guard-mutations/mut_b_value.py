# SPDX-License-Identifier: Apache-2.0

import io, sys
p = sys.argv[1]
s = io.open(p, encoding='utf-8').read()
old = ('\t\t`KC_CLI_PASSWORD="$KC_BOOTSTRAP_ADMIN_PASSWORD" %s config credentials `+\n'
       '\t\t\t`--server http://localhost:8080 --realm master --user admin`, kc)')
new = ('\t\t`%s config credentials `+\n'
       '\t\t\t`--server http://localhost:8080 --realm master --user admin --password %s`, kc, os.Getenv("KC_ADMIN_PASSWORD"))')
if old not in s:
    sys.exit("MUTACAO-NAO-APLICADA")
s = s.replace(old, new, 1)
if '\n\t"os"\n' not in s:
    s = s.replace('import (\n', 'import (\n\t"os"\n', 1)
io.open(p, 'w', encoding='utf-8').write(s)