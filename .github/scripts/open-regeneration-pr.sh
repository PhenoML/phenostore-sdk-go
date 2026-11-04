#!/usr/bin/env bash
# Publish validated spec/client changes from a clean checkout of main.
set -euo pipefail

git add -- api/openapi.yaml phenostore/gen/
if [[ "${INCLUDE_WRAPPER_CHANGES:-false}" == "true" ]]; then
  # The workflow enables this only after Claude adaptation and validation pass.
  git add -- phenostore/ examples/ README.md
fi

# Title generation must not introduce SDK changes after validation.
if [[ -n "${VALIDATED_TREE:-}" && "$(git write-tree)" != "$VALIDATED_TREE" ]]; then
  echo "SDK changes differ from the validated diff; refusing to publish." >&2
  exit 1
fi

if [[ "${1:-}" == "--check" ]]; then
  echo "tree=$(git write-tree)" >> "${GITHUB_OUTPUT:?}"
  if git diff --cached --quiet; then
    echo "changed=false" >> "${GITHUB_OUTPUT:?}"
    echo "SDK and spec already match main."
  else
    echo "changed=true" >> "${GITHUB_OUTPUT:?}"
  fi
  exit 0
fi

: "${GH_REPO:?GH_REPO must identify the SDK repository}"
branch="sdk/regenerate"
title="feat: regenerate SDK from upstream OpenAPI spec"
if [[ -s .commit-message ]]; then
  candidate=$(head -n 1 .commit-message)
  if [[ "$candidate" =~ ^(feat|fix|chore)!?:[[:space:]][^[:space:]].* && ${#candidate} -le 72 ]]; then
    title="$candidate"
  else
    echo "::notice::Invalid generated commit title; using the fallback title."
  fi
fi
rm -f .commit-message

pr_number=$(gh pr list --repo "$GH_REPO" --base main --head "$branch" \
  --state open --json number,isCrossRepository \
  --jq 'map(select(.isCrossRepository == false))[0].number // empty')

if git diff --cached --quiet; then
  echo "SDK and spec already match main."
  if [[ -n "$pr_number" ]]; then
    gh pr close "$pr_number" --repo "$GH_REPO"
  fi
  exit 0
fi

# An empty expected SHA requires a new branch; an existing SHA guards against
# overwriting a concurrent push. Only this automation's branch is replaced.
remote_sha=$(git ls-remote origin "refs/heads/$branch" | cut -f1)
git switch -C "$branch"
git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"
git commit -m "$title"
git push --force-with-lease="refs/heads/$branch:$remote_sha" \
  origin "HEAD:refs/heads/$branch"

body_file=$(mktemp)
trap 'rm -f "$body_file"' EXIT
cat > "$body_file" <<EOF
Regenerates the Go SDK from the upstream Phenostore API contract.

Spec identifier: ${SPEC_SHA:-manual regeneration from the committed spec}
Phenostore commit: ${PHENOSTORE_SHA:-not supplied}

Validation: build and race tests on the SDK's minimum Go version, lint, and generation consistency.
Any required automatic wrapper adaptations are included in this PR and have passed validation.

Review the title before merging: feat: releases a minor version, fix: a patch, feat!: a major, and chore: no release.

PRs created with GITHUB_TOKEN do not automatically trigger PR workflows. Run CI manually on this branch before merging.
EOF

if [[ -n "$pr_number" ]]; then
  gh pr edit "$pr_number" --repo "$GH_REPO" --title "$title" --body-file "$body_file"
else
  gh pr create --repo "$GH_REPO" --base main --head "$branch" \
    --title "$title" --body-file "$body_file"
fi
