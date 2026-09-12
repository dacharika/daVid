# Provenance and submission inventory

daVid is an original repository owned and controlled by its repository owner.
Its authored source comprises the Go orchestration, CLI and MCP surfaces; the
Python media and ML workers; schemas; tests; workflows; and documentation.

The repository does not vendor media, model weights, generated videos, Python
packages, Go modules, or native binaries. Runtime integrations are installed by
the operator and remain outside the repository:

- FFmpeg and ffprobe for media probing and transforms;
- MLT/melt for timeline rendering;
- whisper.cpp and an operator-supplied model for transcription;
- MediaPipe and an operator-supplied face-landmarker model;
- LanceDB plus fastembed or sentence-transformers for local retrieval.

The Git history contains ordinary author identity metadata and `.dacli`
orchestration records. No rewrite is required for the product rename: historical
references to the former cutvec name are retained as development provenance,
while current code and documentation use daVid except where compatibility is
explicitly tested or documented.

Before any external submission, scan the complete history for credentials and
PII, run the documented quality gates from a clean checkout, and verify the
export contains one Git repository.
