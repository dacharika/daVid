#!/usr/bin/env python3
"""cutvec speakers worker — active-speaker detection for a fixed two-person frame.

op: "speakers"
params: {"source": "<path to media>", "layout": "two-person"}
result: {"layout", "turns":[{"speaker":"left"|"right","start","end"}]}

Approach (what actually survives a real interview shot):

* Faces are a small fraction of a wide two-shot, so a whole-frame detector finds
  nothing. Each frame is split into a left and a right half and upscaled before
  detection, which makes each face large enough to land reliably.
* "Who is talking" is mouth *motion*, not mouth openness: the rolling standard
  deviation of MediaPipe's `jawOpen` blendshape per side. A listener holding
  their mouth open never trips it, and — unlike pixel-difference methods — it is
  immune to a moving background (an LED wall, a busy room).
* Raw per-frame decisions are then smoothed into turns: short dropouts are
  bridged, and no turn is emitted shorter than MIN_TURN_SEC.

Analysis runs on a small grayscale-friendly proxy, not the source resolution.
The landmarker model is found via DAVID_FACE_LANDMARKER (or deprecated CUTVEC_FACE_LANDMARKER) or the usual install
locations.
"""

from __future__ import annotations

import os
import tempfile
from pathlib import Path

from cutvec_worker import WorkerError, log, need, require_module, require_tool, run_tool, serve

PROXY_W, PROXY_FPS = 960, 15  # analysis resolution / rate
UPSCALE = 2  # half-crop upscale before detection
WIN_SEC = 0.73  # rolling window for the motion signal
TALK_THRESH = 0.006  # min jawOpen std to count as talking
BOTH_RATIO = 1.35  # winner must beat loser by this ratio
MIN_TURN_SEC = 1.8  # never emit a turn shorter than this
BRIDGE_SEC = 3.5  # bridge dropouts shorter than this

MODEL_CANDIDATES = [
    Path.home() / "Downloads/podcast_edit/models/face_landmarker.task",
    Path.home() / "Library/Application Support/cutvec/models/face_landmarker.task",
    Path(__file__).resolve().parent / "models/face_landmarker.task",
]
MODEL_URL = (
    "https://storage.googleapis.com/mediapipe-models/face_landmarker/"
    "face_landmarker/float16/1/face_landmarker.task"
)


def find_model() -> str:
    env = os.environ.get("DAVID_FACE_LANDMARKER")
    if not env:
        legacy = os.environ.get("CUTVEC_FACE_LANDMARKER")
        if legacy:
            log(
                {
                    "level": "warn",
                    "msg": "CUTVEC_FACE_LANDMARKER is deprecated, rename to DAVID_FACE_LANDMARKER",
                    "deprecated": "CUTVEC_FACE_LANDMARKER",
                    "canonical": "DAVID_FACE_LANDMARKER",
                }
            )
            env = legacy
    if env and Path(env).exists():
        return env
    for cand in MODEL_CANDIDATES:
        if cand.exists():
            return str(cand)
    raise WorkerError(
        "worker.missing",
        "mediapipe face_landmarker.task model not found",
        hint=f"set DAVID_FACE_LANDMARKER, or download it from {MODEL_URL}",
    )


def build_proxy(ffmpeg: str, source: str, dest: str) -> None:
    run_tool(
        [
            ffmpeg,
            "-nostdin",
            "-y",
            "-i",
            source,
            "-vf",
            f"scale={PROXY_W}:-2,fps={PROXY_FPS}",
            "-an",
            "-c:v",
            "mjpeg",
            "-q:v",
            "6",
            dest,
        ],
        "ffmpeg proxy build",
    )


def rolling_std(values, win: int):
    """NaN-robust rolling standard deviation — the 'mouth is moving' signal."""
    np = require_module("numpy", "pip install numpy")
    x = np.asarray(values, dtype=float)
    good = np.isfinite(x)
    filled = np.where(good, x, 0.0)
    out = np.full_like(x, np.nan)
    half = max(1, win // 2)
    for i in range(len(x)):
        lo, hi = max(0, i - half), min(len(x), i + half + 1)
        seg = filled[lo:hi][good[lo:hi]]
        if seg.size >= 3:
            out[i] = seg.std()
    return out


def detect_signals(video: str, model: str):
    """Per-frame jawOpen for the left and right seat, detected on half-crops."""
    cv2 = require_module("cv2", "pip install opencv-python")
    mp = require_module("mediapipe", "pip install mediapipe")
    np = require_module("numpy", "pip install numpy")
    vision = mp.tasks.vision

    landmarker = vision.FaceLandmarker.create_from_options(
        vision.FaceLandmarkerOptions(
            base_options=mp.tasks.BaseOptions(model_asset_path=model),
            running_mode=vision.RunningMode.IMAGE,
            num_faces=1,
            output_face_blendshapes=True,
            min_face_detection_confidence=0.25,
        )
    )
    cap = cv2.VideoCapture(video)
    fps = cap.get(cv2.CAP_PROP_FPS) or PROXY_FPS
    width = int(cap.get(cv2.CAP_PROP_FRAME_WIDTH))
    half = width // 2

    def measure(crop, x_offset):
        """jawOpen plus the face's centre in whole-frame normalised coords."""
        big = cv2.resize(crop, None, fx=UPSCALE, fy=UPSCALE, interpolation=cv2.INTER_LINEAR)
        img = mp.Image(image_format=mp.ImageFormat.SRGB, data=cv2.cvtColor(big, cv2.COLOR_BGR2RGB))
        res = landmarker.detect(img)
        if not res.face_landmarks:
            return np.nan, np.nan, np.nan
        lms = res.face_landmarks[0]
        # Landmarks are normalised within the half-crop, so map x back onto the
        # whole frame: that is what a reframe needs to aim at.
        cx = (x_offset + float(np.mean([p.x for p in lms])) * half) / width
        cy = float(np.mean([p.y for p in lms]))
        jaw = np.nan
        if res.face_blendshapes:
            jaw = next(
                (c.score for c in res.face_blendshapes[0] if c.category_name == "jawOpen"), np.nan
            )
        return jaw, cx, cy

    left, right, i = [], [], 0
    seats = {"left": {"cx": [], "cy": []}, "right": {"cx": [], "cy": []}}
    while True:
        ok, frame = cap.read()
        if not ok:
            break
        jaw_l, cx_l, cy_l = measure(frame[:, :half], 0)
        jaw_r, cx_r, cy_r = measure(frame[:, half:], half)
        left.append(jaw_l)
        right.append(jaw_r)
        for seat, cx, cy in (("left", cx_l, cy_l), ("right", cx_r, cy_r)):
            if np.isfinite(cx):
                seats[seat]["cx"].append(cx)
                seats[seat]["cy"].append(cy)
        i += 1
        if i % 900 == 0:
            log(f"  analysed {i} frames ({i / fps:.0f}s)")
    cap.release()
    landmarker.close()
    if i == 0:
        raise WorkerError(
            "input.invalid",
            "no frames could be read from the media",
            hint="check that the source is a readable video file",
        )
    log(f"analysed {i} frames at {fps:.2f} fps")
    return left, right, fps, summarise_seats(seats, i)


def summarise_seats(seats: dict, frames: int) -> dict:
    """Where each seat actually sits, as the median of its detections.

    The median rather than the mean because a handful of mis-detections would
    drag an average off the subject, and a reframe aimed at the wrong place is
    worse than one aimed at the frame's centre. Seats detected in too few frames
    are omitted so a caller falls back rather than trusting noise.
    """
    np = require_module("numpy", "pip install numpy")
    out = {}
    for seat, vals in seats.items():
        if len(vals["cx"]) < max(10, frames * 0.02):
            continue
        out[seat] = {
            "cx": round(float(np.median(vals["cx"])), 4),
            "cy": round(float(np.median(vals["cy"])), 4),
            "detections": len(vals["cx"]),
        }
    return out


def turns_from_signals(left, right, fps: float) -> list[dict]:
    """Threshold, bridge dropouts, enforce a minimum turn length."""
    np = require_module("numpy", "pip install numpy")
    sig_l = rolling_std(left, max(3, int(WIN_SEC * fps)))
    sig_r = rolling_std(right, max(3, int(WIN_SEC * fps)))

    labels = []
    # strict: the two signals are per-frame and must be the same length; a
    # mismatch would silently shorten the analysed span.
    for a, b in zip(sig_l, sig_r, strict=True):
        a = a if np.isfinite(a) else 0.0
        b = b if np.isfinite(b) else 0.0
        l_on, r_on = a > TALK_THRESH, b > TALK_THRESH
        if l_on and (not r_on or a > BOTH_RATIO * b):
            labels.append("left")
        elif r_on and (not l_on or b > BOTH_RATIO * a):
            labels.append("right")
        else:
            labels.append(None)

    # Hold the last speaker through short gaps; long gaps stay unattributed.
    bridge = int(BRIDGE_SEC * fps)
    filled, last, run = [], None, 0
    for lab in labels:
        if lab is None:
            run += 1
            filled.append(last if last and run < bridge else None)
        else:
            run, last = 0, lab
            filled.append(lab)

    spans, start = [], 0
    for i in range(1, len(filled) + 1):
        if i == len(filled) or filled[i] != filled[start]:
            spans.append([start, i, filled[start]])
            start = i

    min_frames = max(1, int(MIN_TURN_SEC * fps))
    turns = []
    for a, b, who in spans:
        if who is None or (b - a) < min_frames:
            continue
        # extend a contiguous run of the same speaker rather than fragmenting it
        if turns and turns[-1]["speaker"] == who and abs(turns[-1]["end"] - a / fps) < 1e-6:
            turns[-1]["end"] = round(b / fps, 3)
        else:
            turns.append({"speaker": who, "start": round(a / fps, 3), "end": round(b / fps, 3)})
    return turns


def handle(params: dict) -> dict:
    source = need(params, "source")
    layout = params.get("layout") or "two-person"
    if layout != "two-person":
        raise WorkerError(
            "input.invalid",
            f"unsupported layout {layout!r}",
            hint="this worker analyses the fixed 'two-person' side-by-side layout",
        )
    if not Path(source).exists():
        raise WorkerError(
            "input.invalid",
            f"source does not exist: {source}",
            hint="ingest the media first, then run speakers",
        )
    ffmpeg = require_tool("ffmpeg", "brew install ffmpeg")
    model = find_model()

    with tempfile.TemporaryDirectory() as tmp:
        proxy = os.path.join(tmp, "proxy.avi")
        log(f"building {PROXY_W}px/{PROXY_FPS}fps analysis proxy")
        build_proxy(ffmpeg, source, proxy)
        left, right, fps, seats = detect_signals(proxy, model)

    turns = turns_from_signals(left, right, fps)
    talk = {"left": 0.0, "right": 0.0}
    for t in turns:
        talk[t["speaker"]] += t["end"] - t["start"]
    duration = len(left) / fps if fps else 0.0
    attributed = talk["left"] + talk["right"]

    # Detection deliberately declines to attribute ambiguous stretches rather
    # than forcing them onto whoever spoke last. Reporting that share lets a
    # caller judge how much of the edit will fall back to the wide two-shot,
    # and spot a source where detection struggled instead of silently getting
    # a mostly-wide edit.
    coverage = {
        "duration": round(duration, 3),
        "attributed": round(attributed, 3),
        "unattributed": round(max(0.0, duration - attributed), 3),
        "left": round(talk["left"], 3),
        "right": round(talk["right"], 3),
    }
    if duration > 0:
        coverage["attributedFraction"] = round(attributed / duration, 4)

    log(
        f"{len(turns)} turns · left {talk['left']:.0f}s · right {talk['right']:.0f}s "
        f"· unattributed {coverage['unattributed']:.0f}s "
        f"({100 * (1 - attributed / duration):.0f}%)"
        if duration
        else ""
    )
    for seat, pos in seats.items():
        log(
            f"  {seat} seat at x={pos['cx']:.3f} y={pos['cy']:.3f} ({pos['detections']} detections)"
        )
    return {"layout": layout, "turns": turns, "seats": seats, "coverage": coverage}


if __name__ == "__main__":
    serve("speakers", handle)
