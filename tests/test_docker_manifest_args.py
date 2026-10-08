"""The manifest-arg builder must emit `-t <tag>` flags plus per-digest sources.

`docker buildx imagetools create` takes tags as `-t` options and treats positional
arguments as source images, so missing `-t` flags fail with
"can't push with no tags specified".
"""

import subprocess
from pathlib import Path

import yaml

REPO_ROOT: Path = Path(__file__).resolve().parents[1]
SCRIPT: Path = REPO_ROOT / "scripts" / "docker_manifest_args.sh"
IMAGE = "ghcr.io/owner/f1"


def build_args(tmp_path, tags, digests):
    """Run the builder script and return its output split into arguments."""
    digest_dir = tmp_path / "digests"
    digest_dir.mkdir()
    for name in digests:
        (digest_dir / name).touch()
    completed = subprocess.run(
        ["bash", str(SCRIPT), tags, IMAGE, str(digest_dir)],
        capture_output=True,
        text=True,
        check=True,
    )
    return completed.stdout.split()


def test_emits_a_tag_flag_for_every_metadata_tag(tmp_path):
    """Each metadata tag becomes a `-t <tag>` pair."""
    tags = "ghcr.io/owner/f1:main\nghcr.io/owner/f1:latest"

    args = build_args(tmp_path, tags, digests=["aaaa"])

    assert args == [
        "-t", "ghcr.io/owner/f1:main",
        "-t", "ghcr.io/owner/f1:latest",
        "ghcr.io/owner/f1@sha256:aaaa",
    ]


def test_emits_a_source_for_every_digest_file(tmp_path):
    """Each digest file becomes an `<image>@sha256:<digest>` source."""
    tags = "ghcr.io/owner/f1:main"

    args = build_args(tmp_path, tags, digests=["aaaa", "bbbb"])

    assert args == [
        "-t", "ghcr.io/owner/f1:main",
        "ghcr.io/owner/f1@sha256:aaaa",
        "ghcr.io/owner/f1@sha256:bbbb",
    ]


def test_rejects_an_empty_tag_list(tmp_path):
    """An empty tag list must fail loudly instead of pushing nothing."""
    completed = subprocess.run(
        ["bash", str(SCRIPT), "", IMAGE, str(tmp_path)],
        capture_output=True,
        text=True,
        check=False,
    )

    assert completed.returncode != 0
    assert "no tags" in completed.stderr.lower()


def test_rejects_missing_digests(tmp_path):
    """A digest directory with no files must fail loudly."""
    digest_dir = tmp_path / "empty"
    digest_dir.mkdir()
    completed = subprocess.run(
        ["bash", str(SCRIPT), "ghcr.io/owner/f1:main", IMAGE, str(digest_dir)],
        capture_output=True,
        text=True,
        check=False,
    )

    assert completed.returncode != 0
    assert "digest" in completed.stderr.lower()


def test_merge_job_can_reach_the_manifest_builder():
    """The merge job calls scripts/docker_manifest_args.sh, so it must check out the repo.

    A job starts on a fresh runner with an empty workspace, so without a checkout
    step the script is missing, `bash` exits non-zero, and the manifest step fails.
    """
    workflow = yaml.safe_load((REPO_ROOT / ".github" / "workflows" / "docker-publish.yml").read_text())
    steps = workflow["jobs"]["merge"]["steps"]

    assert any("actions/checkout" in (step.get("uses") or "") for step in steps)
    assert any("docker_manifest_args.sh" in (step.get("run") or "") for step in steps)
