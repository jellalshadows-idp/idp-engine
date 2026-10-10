# Runbook: bootstrap an org

Bootstrap is deliberately **outside** the IDP: if the claims repo's protections were managed by claims, a PR could disable the checks that guard it (spec §9.2, ADR-0011).

The commands below are written for **Git Bash** on Windows (or any POSIX shell). PowerShell equivalents are given for `apply` and `check`.

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

## Create the passphrase file

Generate the passphrase and save it in your password manager **first**. Then write it to a file outside any repo, as UTF-8 **without BOM**, on a single line. In Git Bash, this avoids leaving it in your shell history:

```bash
IFS= read -rs P && printf '%s' "$P" > ~/.idp/<name>.pass && unset P
```

`apply` rejects files with a BOM, UTF-16 encoding (the default of PowerShell 5.1 redirection, `>`) or control characters. It never overwrites an existing secret, so a bad value would otherwise persist.

## 3. Apply

```bash
GH_TOKEN="$(gh auth token)" go run ./cmd/idp bootstrap apply \
  --org <org> --claims-repo <claims-repo> --approver <your-login> \
  --reader ~/.idp/apps/<org>-reader.json --writer ~/.idp/apps/<org>-writer.json \
  --passphrase-file ~/.idp/<name>.pass
```

PowerShell (5.1 does not expand `~` for native commands, and neither does Go's flag package, so use `$HOME`):

```powershell
$env:GH_TOKEN = gh auth token
go run ./cmd/idp bootstrap apply `
  --org <org> --claims-repo <claims-repo> --approver <your-login> `
  --reader "$HOME\.idp\apps\<org>-reader.json" --writer "$HOME\.idp\apps\<org>-writer.json" `
  --passphrase-file "$HOME\.idp\<name>.pass"
```

Running it again is safe. A second run prints only `kept existing secret …` lines and `bootstrap apply: done`.

If `apply` fails with a transient error right after creating the repo (GitHub can take a moment to make a new repo's git database available), nothing is broken: re-run `apply`.

## 4. Verify

```bash
GH_TOKEN="$(gh auth token)" go run ./cmd/idp bootstrap check \
  --org <org> --claims-repo <claims-repo> --approver <your-login> \
  --reader ~/.idp/apps/<org>-reader.json --writer ~/.idp/apps/<org>-writer.json
```

PowerShell:

```powershell
$env:GH_TOKEN = gh auth token
go run ./cmd/idp bootstrap check `
  --org <org> --claims-repo <claims-repo> --approver <your-login> `
  --reader "$HOME\.idp\apps\<org>-reader.json" --writer "$HOME\.idp\apps\<org>-writer.json"
```

Expected: `bootstrap check: no drift` and exit code 0.
Drift is reported line by line and exits **3**. A check that could not run (network, token, missing repo access) exits **1** ([ADR-0016](../adr/0016-exit-codes.md)).

Run `check` with an org owner token (or any token with write access to the claims repo). GitHub only returns a ruleset's `bypass_actors` to callers with write access, so with a read-only token `check` reports `$.bypass_actors (not returned to this token; ...)` as drift instead of passing silently (final-review finding I3; the drift-workflow token strategy is decided in Phase 1).

## After bootstrapping

Once every claims repo you intend to bootstrap is done and `check` is clean, delete the local `.pem` files and the passphrase files. The keys and the passphrase now live in GitHub secrets and your password manager.

To bootstrap another claims repo later, generate a new private key in the App's settings page.

## Rotating a secret

Secret values are write-only, so `apply` never overwrites an existing secret. To rotate one:

1. Delete the secret in the repo or environment settings.
2. Run `apply` again.

**Never rotate `IDP_STATE_PASSPHRASE` this way.** The encrypted state in `wet` can only be read with the passphrase it was written with. Rotating it requires an OpenTofu `fallback` migration first (spec §7.2); deleting the secret and re-running `apply` makes the existing state unreadable.

## Changing an App's permissions

The manifests in `bootstrap/apps/*.json` are only read when an App is **created**. Editing a manifest does not change an App that already exists. To apply a new permission set to an existing App:

1. Open `https://github.com/organizations/<org>/settings/apps/<app-slug>/permissions` and set the same permissions as the updated manifest.
2. GitHub then asks the installation to accept the new permissions: open the org's **Settings → GitHub Apps**, select the App and accept the request.
3. Run `check` to confirm nothing else drifted.

## Known gaps

- Until Phase 1 adds the `idp-gate` check, no PR can merge into the claims repo. This is expected.
- Letting Actions create PRs (needed by the `release-please` feature) is enabled in Phase 4, not here.
