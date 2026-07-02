The logins are the per-role operator accounts the toolkit provisions in each entity's Keycloak from the manifest `adminUsers`. Let me verify the NOC/supervisor specifics against the provisioning code, since their login screens differ from the others.

These are the per-role operator accounts the toolkit provisions in each entity's Keycloak from the manifest `adminUsers` (local-profile, ROPC password login). Confirmed against the provisioning code.

### Brazil — spoke-brl

| Portal          | URL                    | Username                      | Password                  |
| --------------- | ---------------------- | ----------------------------- | ------------------------- |
| Governance      | http://localhost:25645 | `admin@brasil.governance.gov` | `brasil-governance-local` |
| Treasury        | http://localhost:26645 | `admin@brasil.treasury.gov`   | `brasil-treasury-local`   |
| Supervisor      | http://localhost:30645 | `admin@brasil.supervisor.gov` | `brasil-supervisor-local` |
| NOC             | http://localhost:32645 | `admin@brasil.noc.gov`        | `brasil-noc-local`        |
| Bank — Itaú     | http://localhost:25646 | `admin@itau.brasil.com`       | `itau-bank-local`         |
| Bank — Bradesco | http://localhost:25647 | `admin@bradesco.brasil.com`   | `bradesco-bank-local`     |

### Colombia — spoke-cop

| Portal             | URL                    | Username                         | Password                    |
| ------------------ | ---------------------- | -------------------------------- | --------------------------- |
| Governance         | http://localhost:25745 | `admin@colombia.governance.gov`  | `colombia-governance-local` |
| Treasury           | http://localhost:26745 | `admin@colombia.treasury.gov`    | `colombia-treasury-local`   |
| Supervisor         | http://localhost:30745 | `admin@colombia.supervisor.gov`  | `colombia-supervisor-local` |
| NOC                | http://localhost:32745 | `admin@colombia.noc.gov`         | `colombia-noc-local`        |
| Bank — Bancolombia | http://localhost:25746 | `admin@bancolombia.colombia.com` | `bancolombia-bank-local`    |
| Bank — Davivienda  | http://localhost:25747 | `admin@davivienda.colombia.com`  | `davivienda-bank-local`     |

### Two notes on the odd ones

- **NOC portal:** its login form pre-fills `noc-admin` / `noc-admin` as a placeholder — that is not the provisioned account. Use the `admin@<country>.noc.gov` credential above (it lands in the shared `cbweb3`/NOC realm with `ROLE_NOC_ADMIN`).
- **Supervisor portal:** the fields are labeled "Client ID / Client Secret," but the provisioned credential is the ROPC user `admin@<country>.supervisor.gov` (role `ROLE_SUPERVISOR`, in the central-bank realm) — enter those in the two fields.

These are local-profile passwords committed in the sample manifests strictly for local development; they are not intended for staging or production.

---

For an accurate mapping I need to see the actual form fields. This is a Scenario A correspondent flow, so let me find the payment/trade-agreement form in the bank portal (Itaú initiates) and identify each field.

There's an `AgreementProposalPage` with an FXAgreement form. Let me read the type and the form to map every field.

This maps to the **New Trade Agreement** form in Itaú's Bank Portal ([AgreementProposalPage.tsx](scenario-a/frontend/apps/bank/src/pages/AgreementProposalPage.tsx)). You'd fill it while logged into Itaú's portal at http://localhost:25646 (`admin@itau.brasil.com`). Itaú is the originator, so it's implicit (derived from your login) — you don't enter it.

Mapping your flow (Itaú → Bradesco on spoke-brl; Bancolombia → Davivienda on spoke-cop):

### Parties

| Form field                | Value                                        | Who                                   |
| ------------------------- | -------------------------------------------- | ------------------------------------- |
| Counterparty B Identity   | `funded_operator@spoke-cop-bank-bancolombia` | Bancolombia (dest-side principal)     |
| Settlement Agent Identity | `funded_operator@spoke-brl-cb`               | Banco Central do Brasil (source CB)   |
| Custodian Identity        | `funded_operator@spoke-cop-bank-bancolombia` | Bancolombia (holds/sends on dest leg) |
| Beneficiary Identity      | `funded_operator@spoke-cop-bank-davivienda`  | Davivienda (final beneficiary)        |
| Source Spoke ID           | `spoke-brl`                                  | Brazil                                |
| Destination Spoke ID      | `spoke-cop`                                  | Colombia                              |
| Source Receiver           | `funded_operator@spoke-brl-bank-bradesco`    | Bradesco (receives source leg)        |
| Destination Receiver      | `funded_operator@spoke-cop-bank-davivienda`  | Davivienda (receives dest leg)        |

### Trade Terms

| Form field       | Value                                     |
| ---------------- | ----------------------------------------- |
| Send Amount      | your BRL amount (e.g. `1000`)             |
| Send Currency    | `BRL`                                     |
| Receive Amount   | your COP amount (e.g. `1250000`)          |
| Receive Currency | `COP`                                     |
| Expiry Date      | leave default (24h) — must be ≥ 5 min out |

The rate auto-computes as `counter / origin`.

Why these pairings: this is the two-leg correspondent model — Itaú (originator) delivers BRL to **Bradesco** (source receiver) on spoke-brl, and **Bancolombia** (custodian/counterparty) delivers COP to **Davivienda** (beneficiary) on spoke-cop, with the **Brazil CB** as settlement agent. So Counterparty B = Custodian = Bancolombia, and Beneficiary = Destination Receiver = Davivienda — matching the form's reference placeholder pattern (counterparty_b == custodian, beneficiary == dest_receiver).

### One blocker to flag

The **Receive Currency** dropdown can't select `COP`. The form hardcodes `CURRENCIES = ["BRL", "EUR", "ARS", "CLP", "MXN", "USD"]` ([AgreementProposalPage.tsx:23](scenario-a/frontend/apps/bank/src/pages/AgreementProposalPage.tsx)) — COP is missing, even though spoke-cop's currency is COP. `BRL` (send side) is fine.

Want me to add `COP` (and `BRL` is already there) to that currency list so you can complete the Colombia leg? It's a one-line change — though note it would need the Itaú bank-frontend image rebuilt to take effect on the running portal.

---

Requests for Initiating and Finishing HTLC PvP

curl 'http://localhost:18646/api/v1/htlc/lock' \
 -H 'sec-ch-ua-platform: "macOS"' \
 -H 'Referer: http://localhost:25646/' \
 -H 'User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10*15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36' \
 -H 'Accept: application/json, text/plain, */\_' \
 -H 'sec-ch-ua: "Chromium";v="149", "Not)A;Brand";v="24"' \
 -H 'Content-Type: application/json' \
 -H 'sec-ch-ua-mobile: ?0' \
 --data-raw '{"receiver":"funded_operator@spoke-brl-bank-bradesco","amount":"1000","agreement_id":"8c1a30a3-23b5-4f3f-8015-736025be7481"}'

{
"contract_id": "cee3cc644b2a8dac294d2953eb1e8c2a358012aba2c62af21c91df44e6f2ff23",
"hash_lock": "59cc46521e5b31098ca0dbdd278532894b1a8941f8e5d9ac1f503a510956c219",
"htlc_tx_hash": "0x501d4311fd3540a8f7d1836b74ee0ef699341272a6350ed55b5361765459a66c",
"zeto_tx_hash": "d505d726-1fda-4bf3-bbd9-fb6d6b144c59",
"secret": "5cd15d6938557b0272572b0f44d7b621d29d814f6401c8aea000ed02900cd8ff"
}

curl 'http://localhost:18646/api/v1/htlc/status/cee3cc644b2a8dac294d2953eb1e8c2a358012aba2c62af21c91df44e6f2ff23' \
 -H 'Accept: application/json, text/plain, _/_' \
 -H 'Accept-Language: en-US,en;q=0.9,pt-BR;q=0.8,pt;q=0.7,pt-PT;q=0.6' \
 -H 'Cache-Control: no-cache' \
 -H 'Connection: keep-alive' \
 -b 'next-auth.csrf-token=3c3a892841c47f230a9686dcb8fb3b5d4db72d9e462cc21adedf406b6d2586f9%7C16215dc3f0ddc82a081cfc959cb76b1d51ed7a9c87e7c576b37dcd3f352aa5d4; onboarding-module-choice=gofabric; next-auth.callback-url=http%3A%2F%2Flocalhost%3A3000%2F; authjs.csrf-token=c50c249352bf49eb14383ade9b90e29f582e977ae1d776fe9c19c022429d3a39%7C586732460db8b42db06d5d628fd70467405044d390e03279da27a145744dc193; **next_hmr_refresh_hash**=b7cac23b5b6ffe26a1f652281713d21c8614fede0f0e6439; authjs.callback-url=http%3A%2F%2Flocalhost%3A3000%2Fsignin; access_token=eyJhbGciOiJSUzI1NiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICJ1RDNZUi1BcnQtbkYyS21SSGRNeW1nd1dseEZUQUdZZDJiTk9HS0w5eGlRIn0.eyJleHAiOjE3ODI5Njg1MTAsImlhdCI6MTc4Mjk2ODIxMCwianRpIjoiYWYyYjcwZjQtNTI5OC00ZTAxLWFjZjItNTg5OTI3YjBlZjI1IiwiaXNzIjoiaHR0cDovL2Nid2ViMy1iYW5rLWl0YXUta2V5Y2xvYWs6ODA4MC9yZWFsbXMvYmFuay1pdGF1Iiwic3ViIjoiMTU3MGQzNDYtM2MxOS00ZTlkLWFhNTItNzZlYzM1YzFlMTVlIiwidHlwIjoiQmVhcmVyIiwiYXpwIjoiYmFuay1pdGF1LWNsaWVudCIsInNpZCI6ImVhYTMxOGY4LWY1MTktNDFjZC1iMWE0LTEwNTk2NzEzOTkyMCIsImFjciI6IjEiLCJhbGxvd2VkLW9yaWdpbnMiOlsiKiJdLCJyZWFsbV9hY2Nlc3MiOnsicm9sZXMiOlsiUk9MRV9CQU5LIl19LCJzY29wZSI6InByb2ZpbGUgZW1haWwiLCJlbWFpbF92ZXJpZmllZCI6dHJ1ZSwibmFtZSI6IkFkbWluIFJPTEVfQkFOSyIsInByZWZlcnJlZF91c2VybmFtZSI6ImFkbWluQGl0YXUuYnJhc2lsLmNvbSIsImdpdmVuX25hbWUiOiJBZG1pbiIsImZhbWlseV9uYW1lIjoiUk9MRV9CQU5LIiwiZW1haWwiOiJhZG1pbkBpdGF1LmJyYXNpbC5jb20ifQ.EjoioLOvj9lXEQTyTJmbzChnt-nu0y0PDCU8qWSJYl7DtDKmLSnjetSXJMlodtZRwNKJWiwb5GNJ5Pziai0_SJk_H-vMdcbIc9UktqshWPAr3U7piMxnZQeXk0ohbe7eIf7ineYpYFmwa7VPJctZrjTS3GR9nw5ezUrBe8x5bsFsvNRy_N-QGaqCQV98S_4LDUnSU0JhKvkBaj3AMbQ0iDMRhmlx0BXbSPLtO4FQcmPVH5OrMBh2A4NNUpvLo0b82a-\_hODx9uy3EAhFbOzCFkTKVh7Y3qp6szXEuTsGjgLKm3Z2eWs5r9pfxYNqTfF7I4WxRg4Z289UvvlnipszTA; refresh_token=eyJhbGciOiJIUzUxMiIsInR5cCIgOiAiSldUIiwia2lkIiA6ICI2ZjJhMjU1NS04YWYwLTRjM2EtOGRkNC05OGI1MGYxMGI0YTYifQ.eyJleHAiOjE3ODI5NzAwMTAsImlhdCI6MTc4Mjk2ODIxMCwianRpIjoiYzlmOWFkZDItMDk1NC00YWNhLTgyOWItOWE5NzdmMjZmNjM0IiwiaXNzIjoiaHR0cDovL2Nid2ViMy1iYW5rLWl0YXUta2V5Y2xvYWs6ODA4MC9yZWFsbXMvYmFuay1pdGF1IiwiYXVkIjoiaHR0cDovL2Nid2ViMy1iYW5rLWl0YXUta2V5Y2xvYWs6ODA4MC9yZWFsbXMvYmFuay1pdGF1Iiwic3ViIjoiMTU3MGQzNDYtM2MxOS00ZTlkLWFhNTItNzZlYzM1YzFlMTVlIiwidHlwIjoiUmVmcmVzaCIsImF6cCI6ImJhbmstaXRhdS1jbGllbnQiLCJzaWQiOiJlYWEzMThmOC1mNTE5LTQxY2QtYjFhNC0xMDU5NjcxMzk5MjAiLCJzY29wZSI6InByb2ZpbGUgYmFzaWMgd2ViLW9yaWdpbnMgcm9sZXMgZW1haWwgYWNyIn0.-KEQtPqcAFbQf8RphQloqvOaa2pUJq5yJm5oMle0d7yNwuD98L1ffiBSvl8_P7OmEPT0y6TzdwhV-uv3zJnyFg' \
 -H 'Origin: http://localhost:25646' \
 -H 'Pragma: no-cache' \
 -H 'Referer: http://localhost:25646/' \
 -H 'Sec-Fetch-Dest: empty' \
 -H 'Sec-Fetch-Mode: cors' \
 -H 'Sec-Fetch-Site: same-site' \
 -H 'User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36' \
 -H 'sec-ch-ua: "Chromium";v="149", "Not)A;Brand";v="24"' \
 -H 'sec-ch-ua-mobile: ?0' \
 -H 'sec-ch-ua-platform: "macOS"'

{
"contract_id": "cee3cc644b2a8dac294d2953eb1e8c2a358012aba2c62af21c91df44e6f2ff23",
"sender": "funded_operator@spoke-brl-bank-itau",
"receiver": "funded_operator@spoke-brl-bank-bradesco",
"hash_lock": "59cc46521e5b31098ca0dbdd278532894b1a8941f8e5d9ac1f503a510956c219",
"time_lock": 1782971828,
"zeto_lock_ref": "0x0be5d377624f8df1f12402738476d95238f9f05a36984be3e24aead5c6d22f08",
"state": "HTLC_STATE_LOCKED",
"counterparty_locked": false
}

curl 'http://localhost:18746/api/v1/htlc/lock-with-hash' \
 -H 'sec-ch-ua-platform: "macOS"' \
 -H 'Referer: http://localhost:25746/' \
 -H 'User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Code/1.126.0 Chrome/148.0.7778.97 Electron/42.2.0 Safari/537.36' \
 -H 'Accept: application/json, text/plain, _/_' \
 -H 'sec-ch-ua: "Not/A)Brand";v="99", "Chromium";v="148"' \
 -H 'Content-Type: application/json' \
 -H 'sec-ch-ua-mobile: ?0' \
 --data-raw '{"hash_lock":"59cc46521e5b31098ca0dbdd278532894b1a8941f8e5d9ac1f503a510956c219","receiver":"funded_operator@spoke-cop-bank-davivienda","amount":"5000","agreement_id":"bf29f0d9-d50e-4b74-8e5a-387b1ac95ed6"}'

{
"contract_id": "39bc350b37de84413349cef6c49e660cce90a6f7fc73d22175a2f99067303150",
"hash_lock": "59cc46521e5b31098ca0dbdd278532894b1a8941f8e5d9ac1f503a510956c219",
"htlc_tx_hash": "0xd3c01b308054564725cb4d8ecf14d0dfc7fe2a33aed101d4008947898ca3b8aa",
"zeto_tx_hash": "4fab32a0-8957-494d-9aea-e1e064d7b1e5"
}

curl 'http://localhost:18646/api/v1/htlc/settle' \
 -H 'sec-ch-ua-platform: "macOS"' \
 -H 'Referer: http://localhost:25646/' \
 -H 'User-Agent: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Safari/537.36' \
 -H 'Accept: application/json, text/plain, _/_' \
 -H 'sec-ch-ua: "Chromium";v="149", "Not)A;Brand";v="24"' \
 -H 'Content-Type: application/json' \
 -H 'sec-ch-ua-mobile: ?0' \
 --data-raw '{"contract_id":"cee3cc644b2a8dac294d2953eb1e8c2a358012aba2c62af21c91df44e6f2ff23","secret":"5cd15d6938557b0272572b0f44d7b621d29d814f6401c8aea000ed02900cd8ff"}'
