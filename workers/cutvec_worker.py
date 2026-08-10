"""Shared plumbing for cutvec's Python ML workers.

Every worker is a subprocess speaking the versioned JSON contract the Go core
defines in internal/worker: exactly one Request JSON document on stdin, exactly
one Response JSON document on stdout.

    Request  {"version":1, "op":"transcribe", "params":{...}}
    Response {"version":1, "result":{...}}
             {"version":1, "error":{"code","message","hint","retryable"}}

Workers must keep stdout clean — anything printed there that is not the single
Response is a protocol violation, so all logging goes to stderr.
"""
from __future__ import annotations

import json
import shutil
import subprocess
import sys
import traceback

CONTRACT_VERSION = 1


class WorkerError(Exception):
    """A structured failure the core maps straight onto its axerror shape."""

    def __init__(self, code: str, message: str, hint: str = "", retryable: bool = False):
        super().__init__(message)
        self.code = code
        self.message = message
        self.hint = hint
        self.retryable = retryable

    def payload(self) -> dict:
        err = {"code": self.code, "message": self.message, "retryable": self.retryable}
        if self.hint:
            err["hint"] = self.hint
        return err


def log(*parts) -> None:
    """Progress/diagnostics go to stderr; stdout carries only the Response."""
    print(*parts, file=sys.stderr, flush=True)


def need(params: dict, key: str) -> str:
    value = params.get(key)
    if not value:
        raise WorkerError(
            "input.invalid",
            f"missing required param {key!r}",
            hint=f'set "{key}" in the request params',
        )
    return value


def require_tool(name: str, install_hint: str) -> str:
    """Resolve an external binary or fail with a non-retryable worker.missing."""
    path = shutil.which(name)
    if not path:
        raise WorkerError(
            "worker.missing",
            f"required tool {name!r} is not on PATH",
            hint=install_hint,
        )
    return path


def require_module(name: str, install_hint: str):
    """Import an optional ML dependency or fail with a clear, actionable error."""
    try:
        return __import__(name)
    except ImportError as exc:
        raise WorkerError(
            "worker.missing",
            f"python package {name!r} is not installed: {exc}",
            hint=install_hint,
        ) from exc


def run_tool(argv: list[str], what: str) -> subprocess.CompletedProcess:
    """Run an external tool, turning a non-zero exit into a retryable error."""
    proc = subprocess.run(argv, capture_output=True, text=True)
    if proc.returncode != 0:
        tail = (proc.stderr or proc.stdout or "").strip().splitlines()[-12:]
        raise WorkerError(
            "worker.crashed",
            f"{what} failed (exit {proc.returncode}): " + " / ".join(tail),
            hint=f"run the {what} command manually to see the full output",
            retryable=True,
        )
    return proc


def serve_ops(handlers: dict) -> None:
    """Serve a worker that answers several ops, dispatching on the request's op.

    Workers that wrap one tool often expose more than one capability — melt both
    renders and reports what filters it has — and keeping them in one executable
    means one thing to install and configure.
    """
    _serve(handlers)


def serve(op_name: str, handler) -> None:
    """Serve a worker that answers exactly one op."""
    _serve({op_name: handler})


def _serve(handlers: dict) -> None:
    """Read the Request, dispatch to the op's handler, write the Response.

    A handler returns the result mapping; any WorkerError becomes a structured
    error response, and an unexpected exception becomes a retryable
    worker.crashed carrying the traceback's last line.
    """
    known = sorted(handlers)
    try:
        raw = sys.stdin.read()
        try:
            req = json.loads(raw) if raw.strip() else {}
        except json.JSONDecodeError as exc:
            raise WorkerError(
                "worker.protocol", f"request is not valid JSON: {exc}",
                hint="the core must write exactly one JSON Request to stdin",
            ) from exc

        version = req.get("version", CONTRACT_VERSION)
        if version != CONTRACT_VERSION:
            raise WorkerError(
                "worker.version",
                f"core speaks protocol version {version}, worker speaks {CONTRACT_VERSION}",
                hint="upgrade the worker or the cutvec core so both speak the same version",
            )
        # A single-op worker keeps answering a request that omits the op.
        got_op = req.get("op") or (known[0] if len(known) == 1 else "")
        handler = handlers.get(got_op)
        if handler is None:
            raise WorkerError(
                "input.invalid",
                f"this worker handles {', '.join(repr(k) for k in known)}, got {got_op!r}",
                hint=f"invoke a worker that implements {got_op!r}" if got_op
                     else f'set "op" to one of: {", ".join(known)}',
            )

        result = handler(req.get("params") or {})
        out = {"version": CONTRACT_VERSION, "result": result}
    except WorkerError as err:
        out = {"version": CONTRACT_VERSION, "error": err.payload()}
    except Exception as exc:  # unexpected: report honestly rather than hang
        log(traceback.format_exc())
        out = {
            "version": CONTRACT_VERSION,
            "error": {
                "code": "worker.crashed",
                "message": f"{type(exc).__name__}: {exc}",
                "hint": "check the worker's stderr for the traceback",
                "retryable": True,
            },
        }
    json.dump(out, sys.stdout)
    sys.stdout.flush()
