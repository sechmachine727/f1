"""In-process stub of an OpenAI-compatible chat completions server.

Test utility only. It records the requests it receives so a test can assert
which endpoint and API path a client actually used.
"""

import json
import threading
from http.server import BaseHTTPRequestHandler
from http.server import ThreadingHTTPServer


class _Handler(BaseHTTPRequestHandler):
    server_version = "StubOpenAI/1"

    # do_POST / do_GET are the names BaseHTTPRequestHandler dispatches to.
    # pylint: disable=invalid-name

    def log_message(self, *args):
        """Silence the default request logging."""

    def do_POST(self):
        """Record the request and answer with a chat completion."""
        length = int(self.headers.get("Content-Length", 0))
        raw = self.rfile.read(length)
        try:
            body = json.loads(raw or b"{}")
        except ValueError:
            body = {}
        self.server.requests.append({
            "path": self.path,
            "authorization": self.headers.get("Authorization"),
            "model": body.get("model"),
            "stream": bool(body.get("stream")),
        })
        if body.get("stream"):
            self._send_stream(body.get("model"))
        else:
            self._send_json(body.get("model"))

    def do_GET(self):
        """Answer any GET with 404; the stub only serves POST."""
        self.send_response(404)
        self.end_headers()

    def _send_stream(self, model):
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.end_headers()
        chunks = [
            {"id": "c1", "object": "chat.completion.chunk", "created": 0, "model": model,
             "choices": [{"index": 0, "delta": {"role": "assistant", "content": "pong"},
                           "finish_reason": None}]},
            {"id": "c1", "object": "chat.completion.chunk", "created": 0, "model": model,
             "choices": [{"index": 0, "delta": {}, "finish_reason": "stop"}]},
            {"id": "c1", "object": "chat.completion.chunk", "created": 0, "model": model,
             "choices": [],
             "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}},
        ]
        for chunk in chunks:
            self.wfile.write(b"data: " + json.dumps(chunk).encode() + b"\n\n")
        self.wfile.write(b"data: [DONE]\n\n")
        self.wfile.flush()

    def _send_json(self, model):
        payload = json.dumps({
            "id": "c1", "object": "chat.completion", "created": 0, "model": model,
            "choices": [{"index": 0, "message": {"role": "assistant", "content": "pong"},
                          "finish_reason": "stop"}],
            "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
        }).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)


class StubOpenAIServer:
    """Threaded OpenAI-compatible stub bound to an ephemeral loopback port."""

    def __init__(self) -> None:
        """Bind the stub and prepare its background thread."""
        self._server = ThreadingHTTPServer(("127.0.0.1", 0), _Handler)
        self._server.requests = []
        self._thread = threading.Thread(target=self._server.serve_forever, daemon=True)

    @property
    def port(self) -> int:
        """The port the stub is bound to."""
        return self._server.server_address[1]

    @property
    def base_url(self) -> str:
        """The OpenAI-style base URL clients should target."""
        return f"http://127.0.0.1:{self.port}/v1"

    @property
    def requests(self) -> list:
        """Requests received so far, newest last."""
        return self._server.requests

    def __enter__(self) -> "StubOpenAIServer":
        self._thread.start()
        return self

    def __exit__(self, *exc_info) -> None:
        self._server.shutdown()
        self._server.server_close()
        self._thread.join(timeout=5)
