# Security Policy

## Supported Versions

Security fixes are applied on a best-effort basis to:

| Version | Supported |
| --- | --- |
| `main` | Yes |
| Latest release | Yes |
| Older releases | No guarantee |

If you are reporting a vulnerability, reproduce it on the latest release or on `main` first when possible.

## Reporting a Vulnerability

Do not open a public GitHub issue for security-sensitive reports.

Prefer private reporting through the repository's security/advisory workflow when available. If private reporting is not available, contact the maintainer directly before disclosing details publicly.

Include:

- A clear description of the issue and its impact
- Affected OS/shell and WUT version
- Exact reproduction steps
- Proof of concept or minimal sample
- Whether the issue requires local access, user interaction, or a crafted file/config

## Sensitive Data

Before sharing logs, configs, or `wut bug-report` archives:

- Remove tokens, credentials, private paths, hostnames, and personal data
- Do not post sensitive artifacts in public issues
- Share the minimum data required to reproduce the issue

## Disclosure Process

- Allow time for triage, validation, and a fix before public disclosure
- Coordinated disclosure is preferred
- If the report is accepted, a fix and release note will be prepared as soon as practical

## Scope

This policy covers vulnerabilities in this repository's code, scripts, and shipped binaries. It does not cover third-party services, packages, or user-specific machine misconfiguration unless WUT is the direct cause.
