"""Exercise regeneration automation with local Git repositories and a fake gh."""

import base64
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


SCRIPTS = Path(__file__).resolve().parents[1]
FALLBACK_TITLE = "feat: regenerate SDK from upstream OpenAPI spec"


def git(directory, *args):
    return subprocess.run(
        ["git", "-C", str(directory), *args],
        check=True, capture_output=True, text=True,
    ).stdout.strip()


class RegenerationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="sdk-regeneration-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.remote = self.root / "remote.git"
        self.checkout = self.root / "checkout"
        git(self.root, "init", "--bare", "--initial-branch=main", str(self.remote))
        git(self.root, "clone", str(self.remote), str(self.checkout))
        git(self.checkout, "config", "user.name", "Test")
        git(self.checkout, "config", "user.email", "test@example.test")
        for name in (
            "api/openapi.yaml", "phenostore/gen/generated.go",
            "phenostore/client.go", "phenostore/client_test.go",
            "examples/basic/main.go", "README.md", "go.mod",
            ".github/workflows/ci.yml",
        ):
            self.write(name, "original\n")
        git(self.checkout, "add", ".")
        git(self.checkout, "commit", "-m", "initial")
        git(self.checkout, "push", "origin", "main")
        fake_bin = self.root / "bin"
        fake_bin.mkdir()
        self.calls_file = self.root / "gh-calls.jsonl"
        fake_gh = fake_bin / "gh"
        fake_gh.write_text(f"#!{sys.executable}\n" + '''
import json
import os
from pathlib import Path
import sys

args = sys.argv[1:]
entry = {"args": args}
if "--body-file" in args:
    entry["body"] = Path(args[args.index("--body-file") + 1]).read_text()
with Path(os.environ["TEST_GH_CALLS"]).open("a") as stream:
    stream.write(json.dumps(entry) + "\\n")
if args[:2] == ["pr", os.environ.get("TEST_GH_FAIL", "")]:
    raise SystemExit(1)
if args[:2] == ["pr", "list"]:
    print(os.environ.get("TEST_PR_NUMBER", ""))
''')
        fake_gh.chmod(0o755)
        self.env = {
            **os.environ,
            "PATH": f"{fake_bin}:{os.environ['PATH']}",
            "GH_REPO": "example/sdk",
            "GH_TOKEN": "fake-test-token",
            "GITHUB_EVENT_NAME": "repository_dispatch",
            "SPEC_SHA": "deadbeef",
            "PHENOSTORE_SHA": "deadbeef01234567",
            "GITHUB_OUTPUT": str(self.root / "output"),
            "TEST_GH_CALLS": str(self.calls_file),
        }

    def write(self, name, content):
        path = self.checkout / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content)

    def run_script(self, name="open-regeneration-pr.sh", *args, **settings):
        return subprocess.run(
            ["bash", str(SCRIPTS / name), *args], cwd=self.checkout,
            env={**self.env, **settings}, capture_output=True, text=True,
        )

    def publish(self, **settings):
        result = self.run_script(**settings)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return result

    def check_changes(self, **settings):
        result = self.run_script("open-regeneration-pr.sh", "--check", **settings)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return dict(
            line.split("=", 1)
            for line in Path(self.env["GITHUB_OUTPUT"]).read_text().splitlines()
        )

    def calls(self):
        if not self.calls_file.exists():
            return []
        return [json.loads(line) for line in self.calls_file.read_text().splitlines()]

    def change(self):
        self.write("api/openapi.yaml", "updated spec\n")
        self.write("phenostore/gen/generated.go", "updated client\n")

    def branch_sha(self):
        return git(self.remote, "rev-parse", "refs/heads/sdk/regenerate")

    def test_no_changes_does_not_publish(self):
        self.publish()
        self.assertEqual([call["args"][1] for call in self.calls()], ["list"])
        self.assertEqual(git(self.remote, "branch", "--list", "sdk/regenerate"), "")

    def test_no_changes_closes_stale_pr(self):
        self.publish(TEST_PR_NUMBER="42")
        self.assertEqual(self.calls()[-1]["args"], [
            "pr", "close", "42", "--repo", "example/sdk",
        ])

    def test_manual_no_changes_preserves_pending_upstream_pr(self):
        self.change()
        self.publish()
        upstream_sha = self.branch_sha()
        git(self.checkout, "switch", "main")
        previous_calls = len(self.calls())
        self.publish(
            TEST_PR_NUMBER="42", GITHUB_EVENT_NAME="workflow_dispatch",
            SPEC_SHA="", PHENOSTORE_SHA="",
        )
        self.assertEqual(self.branch_sha(), upstream_sha)
        self.assertEqual([call["args"][1] for call in self.calls()[previous_calls:]], ["list"])

    def test_no_event_context_does_not_close_pr(self):
        self.publish(TEST_PR_NUMBER="42", GITHUB_EVENT_NAME="")
        self.assertEqual([call["args"][1] for call in self.calls()], ["list"])

    def test_check_ignores_unrelated_and_message_files_without_calling_github(self):
        self.write("README.md", "unrelated edit\n")
        self.write(".commit-message", "chore: temporary message\n")
        outputs = self.check_changes()
        self.assertEqual(outputs["changed"], "false")
        self.assertEqual(self.calls(), [])

    def test_check_includes_new_validated_wrapper_files(self):
        self.write("phenostore/new_test.go", "new regression test\n")
        outputs = self.check_changes(INCLUDE_WRAPPER_CHANGES="true")
        self.assertEqual(outputs["changed"], "true")
        self.assertEqual(self.calls(), [])

    def test_create_commits_only_generated_changes(self):
        main = git(self.remote, "rev-parse", "main")
        self.change()
        self.write("README.md", "unrelated edit\n")
        self.write("go.mod", "unrelated dependency change\n")
        self.publish()
        self.assertEqual(git(self.remote, "rev-parse", "main"), main)
        self.assertEqual(git(self.remote, "show", "sdk/regenerate:api/openapi.yaml"), "updated spec")
        self.assertEqual(git(self.remote, "show", "sdk/regenerate:README.md"), "original")
        self.assertEqual(git(self.remote, "show", "sdk/regenerate:go.mod"), "original")
        request = self.calls()[-1]
        self.assertEqual(request["args"][:2], ["pr", "create"])
        self.assertEqual(request["args"][request["args"].index("--head") + 1], "sdk/regenerate")
        self.assertIn("deadbeef01234567", request["body"])

    def test_update_reuses_branch_and_pr_with_manual_metadata(self):
        self.change()
        self.publish()
        git(self.checkout, "switch", "main")
        self.write("api/openapi.yaml", "newer spec\n")
        self.publish(
            TEST_PR_NUMBER="42", GITHUB_EVENT_NAME="workflow_dispatch",
            SPEC_SHA="", PHENOSTORE_SHA="",
        )
        self.assertEqual(git(self.remote, "show", "sdk/regenerate:api/openapi.yaml"), "newer spec")
        self.assertEqual(sum(call["args"][:2] == ["pr", "create"] for call in self.calls()), 1)
        self.assertEqual(self.calls()[-1]["args"][:3], ["pr", "edit", "42"])
        self.assertIn("manual regeneration", self.calls()[-1]["body"])

    def test_validated_adaptations_and_new_tests_are_included(self):
        self.change()
        for name in ("phenostore/client.go", "phenostore/client_test.go", "README.md"):
            self.write(name, "adapted\n")
        self.write("phenostore/new_test.go", "new regression coverage\n")
        self.write(".github/workflows/ci.yml", "unrelated workflow edit\n")
        self.publish(INCLUDE_WRAPPER_CHANGES="true")
        self.assertEqual(git(self.remote, "show", "sdk/regenerate:phenostore/client.go"), "adapted")
        self.assertEqual(git(self.remote, "show", "sdk/regenerate:phenostore/new_test.go"), "new regression coverage")
        self.assertEqual(git(self.remote, "show", "sdk/regenerate:.github/workflows/ci.yml"), "original")

    def test_wrapper_changes_require_explicit_inclusion(self):
        self.change()
        self.write("phenostore/client.go", "unvalidated change\n")
        self.publish(INCLUDE_WRAPPER_CHANGES="false")
        self.assertEqual(git(self.remote, "show", "sdk/regenerate:phenostore/client.go"), "original")

    def test_generated_title_is_used_without_committing_message_file(self):
        for title in (
            "feat: add resource support", "fix: correct resource decoding",
            "feat!: rename resource parameters", "chore: regenerate internal types",
            "fix: " + "x" * 67,
        ):
            with self.subTest(title=title):
                git(self.checkout, "switch", "main")
                self.change()
                self.write(".commit-message", title + "\n")
                self.publish(CLASSIFICATION_SUCCEEDED="true")
                self.assertEqual(git(self.remote, "log", "-1", "--format=%s", "sdk/regenerate"), title)
                request = self.calls()[-1]["args"]
                self.assertEqual(request[request.index("--title") + 1], title)
                self.assertFalse((self.checkout / ".commit-message").exists())
                self.assertNotIn(".commit-message", git(self.remote, "ls-tree", "--name-only", "sdk/regenerate"))

    def test_invalid_or_empty_title_falls_back(self):
        for title in (
            "", "not conventional", "fix!: unexpected breaking fix",
            "chore!: unexpected release", "fix: " + "x" * 68,
            "fix:\ttab separator", "fix:  leading space",
            "fix: embedded\ttab", "fix: carriage\rreturn",
            "fix: embedded\x1bescape", "fix: delete\x7fcontrol",
            "fix: first line\nchore: second line",
        ):
            with self.subTest(title=title):
                git(self.checkout, "switch", "main")
                self.change()
                self.write(".commit-message", title)
                self.publish()
                self.assertEqual(git(self.remote, "log", "-1", "--format=%s", "sdk/regenerate"), FALLBACK_TITLE)
                request = self.calls()[-1]["args"]
                self.assertEqual(request[request.index("--title") + 1], FALLBACK_TITLE)
                self.assertFalse((self.checkout / ".commit-message").exists())

    def test_failed_classifier_output_is_ignored(self):
        self.change()
        self.write(".commit-message", "feat!: partial classifier output\n")
        self.publish(CLASSIFICATION_SUCCEEDED="false")
        self.assertEqual(git(self.remote, "log", "-1", "--format=%s", "sdk/regenerate"), FALLBACK_TITLE)
        request = self.calls()[-1]["args"]
        self.assertEqual(request[request.index("--title") + 1], FALLBACK_TITLE)
        self.assertFalse((self.checkout / ".commit-message").exists())

    def test_message_only_workspace_does_not_publish(self):
        self.write(".commit-message", "chore: no API changes\n")
        self.publish()
        self.assertFalse((self.checkout / ".commit-message").exists())
        self.assertEqual([call["args"][1] for call in self.calls()], ["list"])

    def test_publishing_uses_the_validated_diff_with_generated_title(self):
        self.change()
        outputs = self.check_changes()
        self.write(".commit-message", "fix: correct resource decoding\n")
        self.publish(VALIDATED_TREE=outputs["tree"])
        self.assertEqual(git(self.remote, "log", "-1", "--format=%s", "sdk/regenerate"), "fix: correct resource decoding")

    def test_changes_after_validation_are_not_published(self):
        self.change()
        outputs = self.check_changes()
        self.write("phenostore/gen/generated.go", "unvalidated change after classification\n")
        result = self.run_script(VALIDATED_TREE=outputs["tree"])
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("refusing to publish", result.stderr)
        self.assertEqual(self.calls(), [])

    def test_lookup_failure_does_not_push(self):
        self.change()
        self.assertNotEqual(self.run_script(TEST_GH_FAIL="list").returncode, 0)
        self.assertEqual(git(self.remote, "branch", "--list", "sdk/regenerate"), "")

    def test_api_creation_failure_can_be_retried(self):
        self.change()
        self.assertNotEqual(self.run_script(TEST_GH_FAIL="create").returncode, 0)
        self.branch_sha()
        git(self.checkout, "switch", "main")
        self.change()
        self.publish()
        self.assertEqual(self.calls()[-1]["args"][:2], ["pr", "create"])

    def test_concurrent_push_is_not_overwritten(self):
        self.change()
        self.publish()
        previous = self.branch_sha()
        self.write("api/openapi.yaml", "concurrent edit\n")
        git(self.checkout, "add", "api/openapi.yaml")
        git(self.checkout, "commit", "-m", "concurrent change")
        concurrent = git(self.checkout, "rev-parse", "HEAD")
        git(self.checkout, "push", "origin", "sdk/regenerate")
        git(self.remote, "update-ref", "refs/heads/sdk/regenerate", previous)
        git(self.checkout, "switch", "main")
        self.change()
        hook = self.checkout / ".git/hooks/pre-commit"
        hook.write_text(
            '#!/bin/sh\nexec git --git-dir="$TEST_REMOTE" update-ref '
            'refs/heads/sdk/regenerate "$TEST_CONCURRENT_SHA"\n'
        )
        hook.chmod(0o755)
        result = self.run_script(
            TEST_PR_NUMBER="42", TEST_REMOTE=str(self.remote),
            TEST_CONCURRENT_SHA=concurrent,
        )
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.branch_sha(), concurrent)
        self.assertEqual(self.calls()[-1]["args"][:2], ["pr", "list"])

    def test_concurrent_branch_creation_is_not_overwritten(self):
        main = git(self.remote, "rev-parse", "main")
        self.change()
        hook = self.checkout / ".git/hooks/pre-commit"
        hook.write_text(
            '#!/bin/sh\nexec git --git-dir="$TEST_REMOTE" update-ref '
            'refs/heads/sdk/regenerate "$TEST_CONCURRENT_SHA"\n'
        )
        hook.chmod(0o755)
        result = self.run_script(TEST_REMOTE=str(self.remote), TEST_CONCURRENT_SHA=main)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(self.branch_sha(), main)
        self.assertEqual(self.calls()[-1]["args"][:2], ["pr", "list"])

    def test_valid_payload_imports_spec(self):
        spec = b"openapi: 3.0.0\n"
        result = self.run_script("import-spec.sh", SPEC_B64=base64.b64encode(spec).decode())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual((self.checkout / "api/openapi.yaml").read_bytes(), spec)

    def test_invalid_payload_preserves_committed_spec(self):
        for settings in (
            {"SPEC_SHA": ""}, {"SPEC_SHA": "../../escape"},
            {"SPEC_SHA": "a" * 65}, {"SPEC_B64": "invalid!"}, {"SPEC_B64": ""},
        ):
            with self.subTest(settings=settings):
                result = self.run_script("import-spec.sh", **settings)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual((self.checkout / "api/openapi.yaml").read_text(), "original\n")


if __name__ == "__main__":
    unittest.main()
