"""End-to-end: a registry-configured custom endpoint is reached via chat completions."""

import asyncio

import pytest
from neuro_san.internals.run_context.langchain.llms.default_llm_factory import DefaultLlmFactory
from registry_config import load_llm_config
from stub_openai_server import StubOpenAIServer

# The stub_server fixture is requested by name, the standard pytest pattern.
# pylint: disable=redefined-outer-name


@pytest.fixture
def stub_server():
    """Run an in-process OpenAI-compatible stub for one test."""
    with StubOpenAIServer() as server:
        yield server


def _create_model(endpoint_config):
    """Build a langchain chat model from a neuro-san llm_config."""
    factory = DefaultLlmFactory()
    factory.load()
    return factory.create_llm(endpoint_config).get_model()


def test_env_override_routes_agents_to_custom_endpoint(stub_server, monkeypatch):
    """LLM_* / OPENAI_API_BASE make agents call the custom server's completions API."""
    monkeypatch.setenv("LLM_CLASS", "openai")
    monkeypatch.setenv("LLM_MODEL_NAME", "my-custom-model")
    monkeypatch.setenv("OPENAI_API_BASE", stub_server.base_url)
    monkeypatch.setenv("OPENAI_API_KEY", "dummy-key")

    model = _create_model(load_llm_config("registries/llm_config.hocon"))
    reply = asyncio.run(model.ainvoke("ping"))

    assert reply.content == "pong"
    received = stub_server.requests
    assert len(received) == 1
    assert received[0]["path"] == "/v1/chat/completions"
    assert received[0]["model"] == "my-custom-model"
    assert received[0]["authorization"] == "Bearer dummy-key"
