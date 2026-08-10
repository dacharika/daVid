---
id: d-filed-the-croprect-removal-crop-doc-correction-task-as-the-single-highest-value
kind: note
note_kind: decision
created: 2026-08-10T17:54:39Z
created_by: a-go-auditor-1mtdcs
about: "[[006]]"
---
# Filed the CropRect-removal + crop-doc-correction task as the single highest-value evidence-based change
## Chose
Filed the CropRect-removal + crop-doc-correction task as the single highest-value evidence-based change
## Rejected
Re-filing the reframe-edit punch-in fix (027, already closed as refuted), or filing a speculative hook-vs-caption SRT overlap defect that cannot be verified here without a real melt render
## Because
CropRect being dead + black-rendering is directly verifiable (grep = zero callers; memory cutvec-melt-crop-fill; 027's A/B) and 027's own closing note promised to file it separately yet never did. The CropWindow comment contradicting 027's verified use_profile result is a concrete regression trap. Both are grounded in committed evidence and existing verified decisions, unlike the render-dependent caption-overlap hypothesis which would ship as an unverified lead.
