# Demo Mode

Run:

```sh
jobscout --demo
```

Demo mode lets someone explore `jobscout` without first creating config files or
a local database.

## Runtime Behavior

In demo mode, `jobscout` does not read or write the normal runtime files:

- `config.yaml`
- `SEARCH_PROMPT.md`
- `jobscout.db`

Those files normally live under the OS-specific user config directory, such as
`~/Library/Application Support/jobscout/` on macOS or
`~/.config/jobscout/` on Linux.

The app uses in-memory config, prompt, job storage, and health cache storage.
Changes made in setup/config menus are kept only for the current session.

## Demo Profile

The built-in profile is a generic two-year software engineer:

- Location: Seattle, WA, US
- Work settings: remote, hybrid, and on-site
- Target titles: Software Engineer, Software Developer, Application Developer,
  Frontend Developer, Backend Developer, and Full Stack Developer
- Role families: frontend, backend, and full stack engineering
- Minimum base compensation: `$85,000 USD`
- Skills and signals: JavaScript, TypeScript, React, Node.js, Go, Python, SQL,
  REST APIs, Git, Docker, AWS basics, unit tests, and CI/CD

The profile excludes manager, lead, senior, staff, principal, director, and
architect-style titles.

## LLM Behavior

LLM features remain available in demo mode. Demo mode does not provide provider
credentials. If a supported provider token is already available in the
environment, the app can use it. Otherwise startup offers the normal choice to
continue without LLM or configure provider auth for the session.

LLM web-search sources are not part of the default demo refresh. Use
`--demo --sources llm_web` to test that path explicitly.
