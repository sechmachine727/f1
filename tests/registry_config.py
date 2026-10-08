"""Helpers for loading the registry HOCON files the way neuro-san does."""

from pathlib import Path

from pyhocon import ConfigFactory

REPO_ROOT: Path = Path(__file__).resolve().parents[1]

# Environment variables that override the committed LLM endpoint settings.
ENDPOINT_ENV_VARS = ("LLM_CLASS", "LLM_MODEL_NAME", "OPENAI_API_BASE")


def load_llm_config(relative_path: str) -> dict:
    """Parse a registry HOCON file and return its ``llm_config`` mapping.

    Includes resolve with the repository root as the base directory, matching
    leaf_common's HOCON loader (``ConfigFactory.parse_string(..., basedir=...)``).
    """
    text = (REPO_ROOT / relative_path).read_text()
    parsed = ConfigFactory.parse_string(text, basedir=str(REPO_ROOT))
    return dict(parsed["llm_config"])
