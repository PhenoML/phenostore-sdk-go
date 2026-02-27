Review the changes on this branch to write an appropriate conventional commit
message for this SDK regeneration.

Run `git diff HEAD` to see the generated and wrapper changes (the spec update
is already committed). Based on the nature of the changes, write a single
conventional commit line to `.commit-message`.

The message must:

- Use a conventional commit prefix: `feat:`, `fix:`, `feat!:`, or `chore:`
  - `feat:` — new API endpoints, new types, new SDK functionality
  - `fix:` — bug fixes, correcting broken behavior
  - `feat!:` — breaking changes to the SDK's public API (removed/renamed exports)
  - `chore:` — internal-only changes with no user-facing impact
- Be concise (under 72 characters)
- Describe what changed from the SDK consumer's perspective, not implementation details

Examples:
- `feat: add Encounter and DocumentReference resource support`
- `feat!: rename SearchParams to QueryParams across all resources`
- `fix: correct Bundle response parsing for empty results`
- `chore: regenerate client with no API changes`
