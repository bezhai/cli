# Personal fork

This fork contains two changes:

- Environment-provided app credentials can mint a tenant access token when no
  explicit token is supplied. Tokens are cached for the current invocation and
  retain their credential source.
- Installing or updating the CLI leaves global agent skills untouched by default.
  Skills installation and synchronization require an explicit `--with-skills`.

## Build and use

The patched version is distributed as source on `fork/explicit-skills`:

```bash
git clone --branch fork/explicit-skills https://github.com/bezhai/cli.git
cd cli
make build
./lark-cli --version
```

The canonical build uses the committed catalog and requires Go 1.23 or later.
To install the binary under your user account:

```bash
make install PREFIX="$HOME/.local"
```

Ensure `$HOME/.local/bin` is on `PATH`. This installation does not install skills.

To update this fork, pull this branch and rebuild:

```bash
git pull --ff-only
make build
# If installed under your user account:
make install PREFIX="$HOME/.local"
```

There is no separate npm package or binary release for this fork. The existing
npm installer and self-update release links still point to upstream; using those
upstream packages replaces the fork's behavior. Use the source build above to
keep these patches.

## Optional skills

Use `lark-cli update --with-skills` to explicitly synchronize official skills.
For a source-installed binary this follows the manual binary-update path; it
synchronizes skills but does not replace the binary. The npm installation wizard
also accepts `install --with-skills` when running this fork's script.

`update --with-skills --skills-layout suite` selects the existing suite layout.
`update --with-skills --force` explicitly reinstalls all official skills.
`update --force` alone only applies to the CLI. Skills sync retains its existing
rules, including installing the full official list on first synchronization.

No existing skills are removed by these changes. `update --json` reports
`"skills_action": "skipped"` when synchronization was not requested. Existing
`--check` behavior remains read-only; it cannot be combined with `--with-skills`.
