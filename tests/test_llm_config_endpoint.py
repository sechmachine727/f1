"""The registry LLM config keeps its default and honors endpoint env overrides."""

import pytest
from registry_config import ENDPOINT_ENV_VARS
from registry_config import load_llm_config

# (registry file, committed default model) for every llm_config in the repo.
LLM_CONFIG_SOURCES = [
    ("registries/llm_config.hocon", "claude-sonnet"),
    ("registries/formula_1_racing_team.hocon", "gpt-5.4-mini"),
]


@pytest.fixture(autouse=True)
def clear_endpoint_env(monkeypatch):
    """Start every test with no endpoint overrides in the environment."""
    for name in ENDPOINT_ENV_VARS:
        monkeypatch.delenv(name, raising=False)


@pytest.mark.parametrize("relative_path,default_model", LLM_CONFIG_SOURCES)
def test_default_model_preserved_without_overrides(relative_path, default_model):
    """Unset overrides leave the committed default alone and force no class."""
    config = load_llm_config(relative_path)
    assert config["model_name"] == default_model
    assert "class" not in config


@pytest.mark.parametrize("relative_path", [path for path, _ in LLM_CONFIG_SOURCES])
def test_openai_compatible_endpoint_overrides(relative_path, monkeypatch):
    """Setting the override vars points the registry at a custom endpoint."""
    monkeypatch.setenv("LLM_CLASS", "openai")
    monkeypatch.setenv("LLM_MODEL_NAME", "my-custom-model")
    monkeypatch.setenv("OPENAI_API_BASE", "http://127.0.0.1:9999/v1")

    config = load_llm_config(relative_path)

    assert config["class"] == "openai"
    assert config["model_name"] == "my-custom-model"
    assert config["openai_api_base"] == "http://127.0.0.1:9999/v1"


def test_partial_override_keeps_default_model(monkeypatch):
    """Overriding only the endpoint URL must not blank out the model."""
    monkeypatch.setenv("OPENAI_API_BASE", "http://127.0.0.1:9999/v1")

    config = load_llm_config("registries/llm_config.hocon")

    assert config["model_name"] == "claude-sonnet"
