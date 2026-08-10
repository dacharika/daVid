---
id: d-shorts-head-and-shoulders-crop-is-an-explicit-source-pixel-9-16-window-via-the
kind: note
note_kind: decision
created: 2026-08-10T17:07:06Z
created_by: a-fixer-pwpjwm
about: "[[023]]"
---
# shorts head-and-shoulders crop is an explicit source-pixel 9:16 window via the mlt crop filter's left/right/top/bottom, computed from Probe dims + seat CX/CY
## Chose
shorts head-and-shoulders crop is an explicit source-pixel 9:16 window via the mlt crop filter's left/right/top/bottom, computed from Probe dims + seat CX/CY
## Rejected
keep center=1 + center_bias (full-body), or use qtcrop/avfilter.crop/qtblend/affine
## Because
Verified against the production melt (workers/melt_worker.py runs 'melt <tl> -consumer avformat'): qtcrop, avfilter.crop, qtblend and affine all render BLACK in this MLT build; crop with use_profile edges is a no-op; only crop with explicit left/right/top/bottom in SOURCE pixels crops-and-fills when the kept region is 9:16. center=1 fills but always keeps full height (full body). An explicit window sized to ~0.7*srcH and centred on the detected seat (CX,CY) with the face in the upper third gives a filling head-and-shoulders shot, verified from an extracted frame.
