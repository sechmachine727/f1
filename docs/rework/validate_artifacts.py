"""Check specification assets; application acceptance is a separate gate."""
from __future__ import annotations

import hashlib
import json
import re
import struct
import subprocess
import xml.etree.ElementTree as ET
from pathlib import Path
from urllib.parse import unquote

ROOT = Path(__file__).resolve().parents[2]
DOCS = ROOT / "docs/rework"
CHANGES = ("go-telemetry-core", "durable-race-history", "telemetry-analysis-exports",
           "deterministic-race-strategy", "telemetry-workspaces", "scheduled-race-engineer",
           "sourced-setup-research", "windows-control-center", "release-qualification")


def main():
    subprocess.run(["node", str(DOCS / "validate_contracts.cjs")], check=True)
    for path in DOCS.rglob("*.json"):
        json.loads(path.read_text())
    for folder, filename, key in (("prompts/v1", "manifest.json", "sha256"),
                                   ("reference", "hashes.json", None)):
        hashes = json.loads((DOCS / folder / filename).read_text())
        if key:
            hashes = hashes[key]
        for name, expected in hashes.items():
            assert hashlib.sha256((DOCS / folder / name).read_bytes()).hexdigest() == expected, name
    spec_files, checked = [], 0
    for name in CHANGES:
        change = ROOT / "openspec/changes" / name
        for filename in ("proposal.md", "design.md", "tasks.md"):
            assert (change / filename).is_file(), f"missing {name}/{filename}"
        specs = list((change / "specs").glob("*/spec.md"))
        assert len(specs) == 1, name
        spec_files.extend(change.rglob("*.md"))
        for block in specs[0].read_text().split("### Requirement: ")[1:]:
            assert "#### Scenario: " in block and "SHALL" in block, name
        for line in (change / "tasks.md").read_text().splitlines():
            if line.startswith("- ["):
                assert re.match(r"- \[[ xX]\] \d+\.\d+ ", line), line
                checked += line.startswith(("- [x]", "- [X]"))
    for path in list(DOCS.rglob("*.md")) + spec_files:
        text = path.read_text()
        assert not re.search(r"\b(TODO|TBD)\b", text), path
        assert "—" not in text, path
        for target in re.findall(r"(?<!!)\[[^\]]+\]\(([^)]+)\)", text):
            if re.match(r"[a-z]+:", target) or target.startswith("#"):
                continue
            assert (path.parent / unquote(target.split("#", 1)[0])).exists(), (path, target)
    for name, dimensions in (("drive-1920x1080.png", (1920,1080)),
                             ("drive-2560x1440.png", (2560,1440)),
                             ("pit-wall-1920x1080.png", (1920,1080))):
        raw = (DOCS / "ui" / name).read_bytes()
        assert raw[:8] == b"\x89PNG\r\n\x1a\n", name
        assert struct.unpack(">II", raw[16:24]) == dimensions, name
    drawing = ET.parse(DOCS / "ui/workspace-layout.drawio")
    assert len(drawing.getroot().findall("diagram")) == 2
    html = (DOCS / "ui/approved-workspaces.html").read_text()
    assert 'id="drive-preview"' in html and 'id="pit-preview"' in html
    assert 'button[aria-pressed]' in html and 'addEventListener("click"' in html
    workload = json.loads((DOCS / "contracts/workload.json").read_text())
    assert sum(workload["base_packets_per_second"].values()) == 1920
    assert workload["base_packets_per_second_total"] + workload["duplicate_packets_per_second"] == 2048
    sizes, rates = workload["wire_payload_sizes"], workload["base_packets_per_second"]
    payload_rate = sum(sizes[k]*v for k,v in rates.items()) + 60*sizes["0"]+20*sizes["1"]+48*sizes["2"]
    assert payload_rate == workload["raw_payload_bytes_per_second"]
    assert workload["healthy_expected_datagrams"] == 2048*7200
    assert workload["healthy_expected_payload_bytes"] == payload_rate*7200
    vectors = json.loads((DOCS / "prompts/v1/scheduler-cases.json").read_text())["cases"]
    assert len(vectors) == len({x["id"] for x in vectors}) and len(vectors) >= 16
    print(f"PASS: 9 OpenSpec artifact sets, links, hashes, screenshots, drawing, fixed workload; {len(vectors)} scheduler vectors; {checked} implementation tasks checked")
    print("Application/provider/production UI qualification is separate; no model calls were made.")


if __name__ == "__main__":
    main()
