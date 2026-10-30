# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/2.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).
A PATCH release with only a base image bump is cut automatically.

## [Unreleased]

## [0.1.0] - 2026-10-30

### Added

- Image `sarak.sh/mikluko/machine-rex`, built on
  `sarak.sh/mikluko/machine-debian:13.20261027.0`, published as `0.1.0`,
  `0.1` and `latest`.
- The Rex server starts as the container command under `dumb-init`, as the
  `rex` account, listening on `/run/rex/server.sock`.
- Claude Code from Anthropic's stable apt channel, at `/usr/bin/claude`.
