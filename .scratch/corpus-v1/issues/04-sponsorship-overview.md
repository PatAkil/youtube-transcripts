# 04: Build the Sponsorship overview

**What to build:** Patrick runs `overview` and gets one Markdown file at the Corpus root answering "which Sponsors work with which Channels?" and "who sponsors this Channel?", with each Video reference carrying the Segment's start time so he can open the read. The overview also shows the agent-pass backlog (Videos with no segments file) and Videos recorded with no Transcript. This ticket fixes the segments file schema that the agent pass (ticket 05) writes.

Spec: `.scratch/corpus-v1/spec.md` (section "Sponsorship overview", segments file under "Corpus layout", `overview` under "Command behaviour"). Glossary: Sponsorship is evidenced by Segments; avoid "deal", "partnership", "ad".

**Blocked by:** 01 (Fetch a Video's Transcript into the Corpus)

**Status:** done

- [x] The segments file is JSON with a processed timestamp and a list of Segments, each with sponsor name, start seconds, end seconds, origin (`slot` or `transcript`), slot index when origin is `slot`, and the opening line of the read. An empty list with a processed timestamp means "processed, no Sponsors found". The CLI parses this shape and the shape is documented in the repo so the agent pass skill can write it.
- [x] `overview` walks every Channel directory and every Video directory beneath it, reading video metadata and segments files.
- [x] The overview Markdown has four sections: **By Sponsor** (Sponsor → Channel → Videos, each Video with the Segment start times), **By Channel** (Channel → Sponsor → Segment count), **Awaiting agent pass** (Videos with no segments file, by Channel), **No transcript** (Videos with transcript status `none`).
- [x] Sponsor names are grouped by trimmed, lower-cased key; the displayed name is the most frequent spelling across Segments.
- [x] Two Segments for the same Sponsor in one Video count as two and both start times appear.
- [x] A segments or video metadata file that fails to parse is reported by name on stderr and skipped; the command still writes the overview and exits 0.
- [x] Running `overview` twice on an unchanged Corpus produces byte-identical output (stable ordering: Sponsors and Channels alphabetical by display name, Videos by publish date).
- [x] Tests build a temporary Corpus from hand-written video metadata and segments files (no upstream calls needed) and assert on the overview text and stderr: multi-Channel grouping, case-insensitive Sponsor merge, same-Sponsor-twice, empty "processed, none found" file appears nowhere in By Sponsor and not in the backlog, missing segments file lands in the backlog, `none` transcript status lands in No transcript, malformed segments file is skipped and named.

## Comments

- Done on branch `ticket/04-sponsorship-overview`. Schema types and parser in `internal/corpus/segments.go`, documented in `docs/segments-file.md` for ticket 05; command in `internal/cli/overview.go`.
- Each Segment start renders as `[[mm:ss]](https://www.youtube.com/watch?v=<id>&t=<s>s)`: the glossary timestamp as link text, opening the Video at the read.
- A Video with transcript `none` and no segments file is listed under No transcript only, not in the agent-pass backlog (nothing for the agent to read). The stdout summary is `wrote <path>  sponsors=N  channels=N  awaiting=N`.
