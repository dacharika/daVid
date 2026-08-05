---
id: d-vector-search-lancedb-local-embeddings-fastembed-sentence-transformers
kind: note
note_kind: decision
created: 2026-08-05T13:27:09Z
created_by: a-root
---
# Vector search = LanceDB + local embeddings (fastembed/sentence-transformers)
## Chose
Vector search = LanceDB + local embeddings (fastembed/sentence-transformers)
## Rejected
Chroma; API embeddings
## Because
embedded, file-based, fully offline, no API keys; an agent tool must run deterministically without network.
