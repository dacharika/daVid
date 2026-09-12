# daVid Python workers

The Go core does orchestration, caching and the agent-facing contract; these
workers do the ML and media work behind it. Each one is a subprocess speaking
the versioned JSON contract in `internal/worker`: exactly one Request on stdin,
exactly one Response on stdout, with all logging on stderr.

```
Request  {"version":1,"op":"transcribe","params":{...}}
Response {"version":1,"result":{...}}
         {"version":1,"error":{"code","message","hint","retryable"}}
```

| Worker | op | params | result |
|---|---|---|---|
| `ffmpeg_worker.py` | `probe` | `source` | `width`, `height`, `duration`, `fps` |
| `whisper_worker.py` | `transcribe` | `source` | `language`, `text`, `words[{word,start,end}]` |
| `mediapipe_worker.py` | `speakers` | `source`, `layout` | `layout`, `turns[{speaker,start,end}]` |
| `index_worker.py` | `index` | `transcript`, `table` | `source`, `table`, `segments`, `dim`, `model` |
| `search_worker.py` | `search` | `query`, `table`, `limit` | `candidates[{start,end,text,score}]`, `model` |
| `melt_worker.py` | `render` | `timeline`, `output`, `quality` | `output`, `quality`, `bytes` |

## Requirements

System tools: `ffmpeg` (all media work), `whisper-cli` from whisper.cpp
(transcribe), `melt` from MLT (render).

```bash
brew install ffmpeg whisper-cpp mlt
pip install lancedb fastembed mediapipe opencv-python numpy
```

Models are found automatically where whisper.cpp/podcli install them, or set
them explicitly:

- `DAVID_WHISPER_MODEL` — a ggml whisper model (`ggml-base.bin`)
- `DAVID_FACE_LANDMARKER` — MediaPipe `face_landmarker.task`
- `DAVID_EMBED_MODEL` — embedding model (default `BAAI/bge-small-en-v1.5`)

Legacy `CUTVEC_*` names remain read-only compatibility aliases for one
deprecation window. `DAVID_*` always wins when both forms are present; daVid
emits a structured warning when it observes a legacy name. New integrations
must use `DAVID_*`.

Embeddings run **locally** (fastembed, falling back to sentence-transformers).
No API key, no network at query time — an agent tool has to be reproducible.

## Pointing the core at a worker

Each command takes the executable in its request, or falls back to an env var:

```bash
PY=/path/to/python3   # the interpreter that has the ML packages installed
export DAVID_WHISPER_WORKER=$PY   # + DAVID_FFMPEG_WORKER, DAVID_INDEX_WORKER,
                                   #   DAVID_SEARCH_WORKER, DAVID_MEDIAPIPE_WORKER,
                                   #   DAVID_MELT_WORKER
```

Per-request (this is what the smoke test does), which also carries the script path:

```bash
echo '{"project":"/tmp/p","worker":"'$PY'","workerArgs":["workers/index_worker.py"]}' \
  | daVid index /tmp/p --json
```

## Verifying

`./workers/smoke_test.sh <media-file>` runs the whole pipeline — init, ingest,
transcribe, index, search, speakers — against real media and fails loudly if a
step stops doing real work. Results are cached by content hash, so a second run
is a cache hit that never starts a worker.

## Design notes

**Active-speaker detection** measures mouth *motion* (rolling standard deviation
of MediaPipe's `jawOpen`), not openness, on per-side half-crops of the frame.
Faces in a wide two-shot are too small for whole-frame detection, and motion —
unlike pixel differencing — ignores a moving background such as an LED wall.

**Indexing** groups word-level tokens into overlapping 4–12s segments. Words are
too fine to retrieve on; segments are long enough to hold a thought and short
enough that a hit is already a usable cut point.
