"""Local text embeddings for cutvec's index/search workers.

Embeddings are deliberately local: an agent tool must run offline and
deterministically, with no API key and no network at query time. fastembed
(ONNX runtime) is preferred because it is light and fast on CPU;
sentence-transformers is accepted as a fallback when it is what the machine has.

Both workers must embed with the *same* backend and model, otherwise query
vectors land in a different space than the indexed ones — so the model name is
written into the index and echoed back on search.
"""

from __future__ import annotations

import os

from david_worker import WorkerError, log


def _embed_model() -> str:
    if v := os.environ.get("DAVID_EMBED_MODEL"):
        return v
    if v := os.environ.get("CUTVEC_EMBED_MODEL"):
        log(
            {
                "level": "warn",
                "msg": "CUTVEC_EMBED_MODEL is deprecated, rename to DAVID_EMBED_MODEL",
                "deprecated": "CUTVEC_EMBED_MODEL",
                "canonical": "DAVID_EMBED_MODEL",
            }
        )
        return v
    return "BAAI/bge-small-en-v1.5"


DEFAULT_MODEL = _embed_model()

INSTALL_HINT = (
    "pip install fastembed  (or sentence-transformers) — cutvec embeds locally, no API key required"
)


class Embedder:
    """A local embedding backend: .encode(texts) -> list[list[float]]."""

    def __init__(self, name: str, model: str, encode):
        self.backend = name
        self.model = model
        self._encode = encode

    def encode(self, texts: list[str]) -> list[list[float]]:
        vectors = self._encode(texts)
        return [list(map(float, v)) for v in vectors]

    @property
    def label(self) -> str:
        return f"{self.backend}:{self.model}"


def load(model_name: str = DEFAULT_MODEL) -> Embedder:
    """Load fastembed if present, else sentence-transformers, else fail clearly."""
    try:
        from fastembed import TextEmbedding  # type: ignore

        log(f"embedding with fastembed model={model_name}")
        embed = TextEmbedding(model_name=model_name)
        return Embedder("fastembed", model_name, lambda texts: list(embed.embed(texts)))
    except ImportError:
        pass

    try:
        from sentence_transformers import SentenceTransformer  # type: ignore

        st_name = model_name.split("/")[-1] if "/" in model_name else model_name
        log(f"embedding with sentence-transformers model={st_name}")
        model = SentenceTransformer(st_name)
        return Embedder(
            "sentence-transformers",
            st_name,
            lambda texts: model.encode(texts, show_progress_bar=False),
        )
    except ImportError:
        pass

    raise WorkerError(
        "worker.missing",
        "no local embedding backend is installed (tried fastembed, sentence-transformers)",
        hint=INSTALL_HINT,
    )
