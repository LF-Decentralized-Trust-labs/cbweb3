## What does this change?

<!-- Describe the change and why it is needed. Link the issue it closes. -->

Closes #

## Type of change

- [ ] Documentation
- [ ] Toolbox artifact (interface contract, mock, test vector, conformance test)
- [ ] Governance
- [ ] CI / tooling
- [ ] Platform code
- [ ] Fix

## Checklist

- [ ] Commits are signed off (`git commit -s`) — DCO is enforced
- [ ] Changes are focused and reviewable; large work is split across pull requests
- [ ] New source files carry `SPDX-License-Identifier: Apache-2.0`
- [ ] No secrets, private keys, credentials or real institutional data are included
- [ ] Documentation updated where behaviour or interfaces changed
- [ ] CI passes

## Breaking changes to published interfaces

<!-- Toolbox/contracts/** is what other implementers build against. If you changed it: -->

- [ ] Not applicable
- [ ] Version bumped and `CHANGELOG.md` updated in the contract directory
- [ ] Mocks, test vectors and conformance tests updated to match

## AI assistance

Per the draft [LFDT AI Guidelines](https://github.com/LF-Decentralized-Trust/governance/pull/321):

- [ ] No AI assistance was used
- [ ] AI assistance was used and disclosed with an `Assisted-by:` trailer

If AI was used, confirm:

- [ ] I reviewed the output and understand every line I am submitting
- [ ] No AI is listed as an author or co-author, and no AI added the `Signed-off-by` trailer
