---
id: f-timeline-build-never-sequences-clips-unless-audio-is-present
kind: note
note_kind: finding
created: 2026-08-10T18:46:31Z
created_by: a-go-auditor-emr68x
about: "[[006]]"
severity: major
---
# timeline build never sequences clips unless audio is present
timelineBuildCmd (cmd/cutvec/main.go:229-256) builds a multi-clip timeline by appending Producers, resolving/applying effects, and applying audio, but NEVER calls tl.Sequence(). Sequencing — the <playlist>+<tractor> that orders clips into one cut sequence — happens ONLY as a side effect inside AddAudio (internal/timeline/timeline.go:92-96: 'if t.Tractor == nil { t.Sequence("main") }'), i.e. only when req.Audio is non-empty. So a 'timeline' build with 2+ clips and NO audio writes bare <producer> elements with no <playlist> and no <tractor>: the clips are never wired into an ordered edit. Timeline.Validate() (timeline.go:379-404) checks producer ids/resources/mlt_service only, never that a multi-producer timeline is sequenced, so the document 'validates' yet does not play as an edit. Provable in Go with no melt render: after building a 2-clip no-audio request, tl.Playlists is empty and tl.Tractor is nil. Latent/green: TestTimelineSlice (cmd/cutvec/main_test.go:301-342) builds exactly the 2-clip no-audio case and asserts only that the XML contains both resources + producer ids — it never checks for a playlist/tractor, so the defect ships green. Contrast: reframe.go:156 and buildShortTimeline both call Sequence explicitly, and TestSequenceWiresEveryProducerIntoAPlaylist (reframe_test.go:102-116) shows Sequence is what makes a multi-producer timeline play as one edit. Impact: the core 'clips play as one edit' promise is broken on the most basic call shape (multi-clip, no music bed); a single clip masks it since a lone producer is renderable. Distinct from open task 029 (hook/caption SRT overlap) and 030 (audio bed gain default).
