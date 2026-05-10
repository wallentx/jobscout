# Release

Use the repo Makefile for release preparation.

## Preflight

```sh
make all
```

For a heavier local pass, run:

```sh
make full-check
```

## Versioning

Release builds embed the value of `VERSION` into `jobscout --version`.

By default, `VERSION` comes from `RELEASE_TAG` when the CI toolbox action
provides it. Otherwise, local dev builds use:

```text
<latest-tag-or-v0.0.0>-<git-short-sha>
```

For example, `v0.1.0-a1b2c3d`.

## First Public Release

Use `v0.1.0` for the first public release.

```sh
make all
git tag -a v0.1.0 -m "jobscout v0.1.0"
git push origin main
git push origin v0.1.0
```

Pushing the tag starts the release workflow. The workflow runs `make all`,
builds release archives for Linux, macOS, and Windows on amd64 and arm64, then
creates or updates the GitHub Release for that tag with archives and checksums.
Linux and macOS assets are `.tar.gz` archives; Windows assets are `.zip`
archives.

For a local release build, pass the release version explicitly:

```sh
VERSION=v0.1.0 make release
```

For the same version source the release workflow uses:

```sh
RELEASE_TAG=v0.1.0 make release
```

## Archives

`make release` writes an archive under `dist/` for the selected target:

```sh
make release RELEASE_GOOS=linux RELEASE_GOARCH=amd64
make release RELEASE_GOOS=darwin RELEASE_GOARCH=arm64
make release RELEASE_GOOS=windows RELEASE_GOARCH=amd64
```

Each archive contains the `jobscout` binary, `README.md`, and `LICENSE`. The
Windows archive contains `jobscout.exe`.

## Install Check

From a checkout:

```sh
make install VERSION=v0.1.0
jobscout --version
```
