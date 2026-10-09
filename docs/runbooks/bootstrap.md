# Runbook: bootstrap an org

Bootstrap is deliberately **outside** the IDP: if the claims repo's protections were managed by claims, a PR could disable the checks that guard it (spec §9.2, ADR-0011).

## Prerequisites

- A GitHub **Free** organization you own.
- `gh` logged in as an org owner, **with the `admin:org` scope**:
  `gh auth refresh -h github.com -s admin:org`
- Go 1.26, to run the CLI from a checkout: `go run ./cmd/idp ...`
- A passphrase of at least 16 characters, saved in your password manager **first**. If you lose it, the encrypted state in `wet` can no longer be read.

## 1. Create the two Apps

The names are `<org>-reader` and `<org>-writer`. GitHub App names are unique across GitHub and limited to 34 characters.

```bash
go run ./cmd/idp bootstrap app --org <org> --role reader
go run ./cmd/idp bootstrap app --org <org> --role writer
```

For each one, open the printed `http://127.0.0.1:<port>/` link and confirm **Create GitHub App** on GitHub. The credentials land in `~/.idp/apps/<slug>.json` and `<slug>.pem`. **Never commit them.**

On Windows, file modes such as 0600 are not enforced. `~/.idp/apps` lives in your user profile, which Windows restricts to your account by default — never move these files to a shared folder.

## 2. Install both Apps

Open each printed `https://github.com/apps/<slug>/installations/new` link, choose the org, and select **All repositories**. The writer creates repositories, so it needs org-wide access.

## 3. Apply

```bash
GH_TOKEN="$(gh auth token)" go run ./cmd/idp bootstrap apply \
  --org <org> --claims-repo <claims-repo> --approver <your-login> \
  --reader ~/.idp/apps/<org>-reader.json --writer ~/.idp/apps/<org>-writer.json \
  --passphrase-file <file>
```

Running it again is safe. A second run prints only `kept existing secret …` lines and `bootstrap apply: done`.

## 4. Verify

```bash
GH_TOKEN="$(gh auth token)" go run ./cmd/idp bootstrap check \
  --org <org> --claims-repo <claims-repo> --approver <your-login> \
  --reader ~/.idp/apps/<org>-reader.json --writer ~/.idp/apps/<org>-writer.json
```

Expected: `bootstrap check: no drift` and exit code 0.

## Rotating a secret

Secret values are write-only, so `apply` never overwrites an existing secret. To rotate one:

1. Delete the secret in the repo or environment settings.
2. Run `apply` again.

**Never rotate `IDP_STATE_PASSPHRASE` this way.** The encrypted state in `wet` can only be read with the passphrase it was written with. Rotating it requires an OpenTofu `fallback` migration first (spec §7.2); deleting the secret and re-running `apply` makes the existing state unreadable.

## Known gaps

- Until Phase 1 adds the `idp-gate` check, no PR can merge into the claims repo. This is expected.
- Letting Actions create PRs (needed by the `release-please` feature) is enabled in Phase 4, not here.
