---
id: d-go-core-cli-mcp-orchestrates-python-workers-for-all-ml
kind: note
note_kind: decision
created: 2026-08-05T13:27:09Z
created_by: a-root
---
# Go core (CLI + MCP) orchestrates Python workers for all ML
## Chose
Go core (CLI + MCP) orchestrates Python workers for all ML
## Rejected
pure Python CLI; pure Go
## Because
Go gives a single-binary CLI+MCP with clean AX and easy distribution; whisper, mediapipe, embeddings and MLT tooling are Python-native. Boundary is subprocess JSON contracts.
