# Security Policy

## Supported Versions

Only the latest released minor version receives security fixes. Older
versions may be patched at the maintainers' discretion.

| Version | Supported          |
| ------- | ------------------ |
| latest  | :white_check_mark: |
| older   | :x:                |

## Reporting a Vulnerability

Please **do not** open a public GitHub issue for security vulnerabilities.

Use one of the following private channels instead:

1. **GitHub Security Advisory** (preferred): open a private advisory at
   <https://github.com/xueqianLu/rpcduel/security/advisories/new>.
2. **Email**: reach out to the maintainer listed in the repository's
   GitHub profile.

When reporting, please include:

- A clear description of the vulnerability and its impact.
- Steps to reproduce, or a proof-of-concept.
- The affected version(s) of `rpcduel`.
- Any suggested mitigation, if known.

You can expect an initial response within **5 business days**. We will
work with you on a coordinated disclosure timeline (typically up to
90 days) and credit you in the release notes unless you prefer to
remain anonymous.

## Scope

In scope:

- Bugs in the `rpcduel` CLI and library code that allow:
  - Arbitrary code execution.
  - Reading or modifying files outside the working directory without
    user consent.
  - Bypassing TLS verification for outbound RPC requests.
  - Leaking secrets supplied via flags / headers / environment.

Out of scope:

- Denial-of-service caused by intentionally hostile RPC endpoints
  (rate-limit responses, malformed payloads). `rpcduel` is a client
  tool; treat its targets as adversarial in your own setup.
- Vulnerabilities in third-party dependencies that have already been
  reported upstream.
