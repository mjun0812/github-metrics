# Security policy

## Supported versions

Only the latest release on [GitHub Releases](https://github.com/mjun0812/github-metrics/releases) receives security fixes. Users on older `vX.Y.Z` tags should upgrade to consume patches.

## Reporting a vulnerability

**Do not open a public issue for security problems.**

Use GitHub's private vulnerability reporting to disclose the issue:

<https://github.com/mjun0812/github-metrics/security/advisories/new>

If the private-reporting flow is unavailable, email the maintainer at **junya.m0812@gmail.com** with a clear subject line (e.g. `[github-metrics] security: <one-line summary>`).

Please include:

- A description of the vulnerability and its impact.
- Reproduction steps, proof-of-concept, or a minimal patch that demonstrates the issue.
- The affected version (`metrics-cli --version` or the Docker image digest).
- Whether the issue is already public elsewhere.

You should get an acknowledgement within **7 days**. A fix or mitigation is targeted within **30 days** of a confirmed report; complex issues may take longer and will be communicated in the acknowledgement thread.

## Scope

This project reads GitHub API data through a user-supplied token and renders SVG / PNG / JPEG / JSON. Areas of particular interest for security reports:

- **Token handling** — anything that could leak the caller's `GITHUB_TOKEN` (via logs, output artifacts, error messages, or CI matrix outputs).
- **Rendered output** — SVG / JSON generation that could inject attacker-controlled markup into a downstream consumer's page.
- **Release supply chain** — cosign signature bypass, tampering with the `SHA256SUMS` file, or GHCR image manifest issues.
- **CI / release workflows** — anything that would let a public PR gain unintended permissions or push to `main` / `ghcr.io`.

Out of scope:

- Denial-of-service against the GitHub API (rate-limit exhaustion is a documented plugin trade-off).
- Vulnerabilities in `lowlighter/metrics` unrelated to code in this repository — report those to <https://github.com/lowlighter/metrics/security>.
- Vulnerabilities in transitive Go dependencies — please open the report upstream and reference it here so we can bump the pin.

## Disclosure

Public disclosure is coordinated after a fix ships. A GitHub Security Advisory will be published on the repository, and the fix release notes will link back to the advisory.
