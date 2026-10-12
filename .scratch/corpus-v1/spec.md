# Corpus v1: fetch Transcripts, take Slots from SponsorBlock, sync Followed Channels, build the Sponsorship overview

Status: ready-for-agent

## Problem Statement

Patrick runs a tech YouTube channel with paid sponsor reads and has two recurring jobs that both need the same raw material:

1. **Outreach.** When looking for new Sponsors, Patrick wants to know which Sponsors already partner with Channels of a similar size and audience. Today that means opening video after video, scrubbing to the ad, and keeping notes by hand.
2. **Writing a read.** When drafting his own sponsor read, Patrick wants to see how other creators phrase and place theirs: where in the video it lands, how long it runs, how they transition in and out.

Nothing in the repo does any of this yet. There is no Corpus, no way to pull a Transcript, and no record of who sponsors whom.

## Solution

A small deterministic Go CLI builds and maintains the Corpus on disk, and an agent skill in this repo does the judgment work on top of it (per ADR-0001).

- `fetch` takes one Video (ID or URL) and stores its Transcript from YouTube's English caption track plus its Sponsor Slots from SponsorBlock.
- `sync` walks every Followed Channel, finds Videos not yet in the Corpus, and fetches them.
- The **agent pass** skill reads a Video's Slots and Transcript, names the Sponsor for each read, splits a Slot holding two reads into two Sponsor Segments, finds reads in Videos SponsorBlock knows nothing about, and writes the Video's Segments file.
- `overview` reads every Segments file and writes the Sponsorship overview: which Sponsors appear on which Channels, in which Videos, and which Videos are still waiting for an agent pass.

Everything lives as plain files committed to the repo, so the Corpus grows over time and is readable by both Patrick and an agent without a database.

## User Stories

### Fetching one Video

1. As Patrick, I want to fetch a Video by its YouTube ID, so that its Transcript lands in the Corpus without me copying captions by hand.
2. As Patrick, I want to fetch a Video by pasting any YouTube URL form (watch, youtu.be, shorts, with extra query parameters), so that I don't have to extract the ID myself.
3. As Patrick, I want the Transcript to come from YouTube's English caption track, so that the words and timestamps are the ones viewers actually see.
4. As Patrick, I want a manually uploaded English track preferred over an auto-generated one when both exist, so that the Transcript is as accurate as possible.
5. As Patrick, I want an auto-generated English track used when no manual one exists, so that most Videos still get a Transcript.
6. As Patrick, I want a Video with no English caption track at all recorded in the Corpus with a clear "no transcript" marker, so that I know it was tried and don't retry it by accident.
7. As Patrick, I want every Transcript line stored with its start time, so that a Segment can be located in the Video and the read's length can be measured.
8. As Patrick, I want the Transcript stored as readable timestamped text rather than structured data, so that I and the agent can read a sponsor read the way it was spoken.
9. As Patrick, I want the Video's metadata (Channel ID, Channel name, title, publish date, fetch date) stored next to the Transcript, so that the overview can group by Channel without a second lookup.
10. As Patrick, I want the fetch to also ask SponsorBlock for the Video's sponsor windows and store them as Sponsor Slots, so that the agent pass can skip straight to the relevant part of the Transcript.
11. As Patrick, I want only SponsorBlock's `sponsor` category treated as Slots, so that self-promotion and "like and subscribe" reminders don't pollute the Corpus (they are not Sponsor Segments).
12. As Patrick, I want overlapping or touching SponsorBlock windows merged into one Slot, so that a Slot is the contiguous window the glossary describes.
13. As Patrick, I want a Video that SponsorBlock has no data on recorded with an empty Slots list, so that the agent pass knows to read the whole Transcript.
14. As Patrick, I want fetching a Video that is already in the Corpus to be a no-op by default, so that re-running commands is safe and cheap.
15. As Patrick, I want a `--force` option to refetch a Video's Transcript and Slots, so that I can refresh a Video whose captions were corrected.
16. As Patrick, I want a refetch to never touch an existing Segments file, so that the agent's work is never destroyed by the CLI.
17. As Patrick, I want the fetch to print one summary line per Video (ID, Channel, title, number of Transcript lines, number of Slots), so that I can see at a glance what was stored.
18. As Patrick, I want a failed fetch to exit non-zero with the reason on stderr, so that I can tell a network error from a Video with no captions.

### Following Channels and syncing

19. As Patrick, I want to keep a committed list of Followed Channels in the Corpus, so that the set of Channels I study is versioned alongside the Transcripts.
20. As Patrick, I want to add a Followed Channel by editing that list by hand with the channel ID and a display name, so that I don't need a separate command for it.
21. As Patrick, I want `sync` to walk every Followed Channel and pull its newest Videos, so that the Corpus keeps up with the Channels I follow.
22. As Patrick, I want `sync` to fetch only Videos not already in the Corpus, so that a sync is fast and never re-downloads what I have.
23. As Patrick, I want a `sync` that works without any YouTube API key, so that the CLI stays key-free as ADR-0001 requires.
24. As Patrick, I want one failing Video (or one failing Channel) to be reported and skipped rather than abort the whole sync, so that one bad Video doesn't block the rest.
25. As Patrick, I want `sync` to end with a summary (Channels walked, Videos fetched, Videos skipped as already present, Videos failed), so that I know what changed.
26. As Patrick, I want `sync` to be safe to run repeatedly, so that I can run it whenever I sit down to work without thinking about state.
27. As Patrick, I want a Followed Channel whose feed can't be read to be named in the summary, so that I can fix a typo in the channel ID.

### The agent pass (Sponsor Segments)

28. As Patrick, I want an agent skill in this repo that performs the agent pass on one Video or on every Video that doesn't have a Segments file yet, so that the judgment work runs on demand.
29. As the agent, I want to read a Video's Slots and only the Transcript lines inside each Slot (with a small margin either side), so that I can name the Sponsor without reading the whole Video.
30. As the agent, I want to read the whole Transcript when the Video has no Slots, so that Videos SponsorBlock doesn't cover still get their Sponsor Segments found.
31. As the agent, I want to split a Slot that holds back-to-back reads for two Sponsors into two Sponsor Segments, so that each Sponsor gets its own entry.
32. As the agent, I want to record two reads for the same Sponsor in one Video as two Segments, so that placement patterns (intro read plus mid-roll read) are visible.
33. As the agent, I want to leave out self-promotion (the creator's own course, merch, Patreon) and engagement reminders, so that only paid Sponsors appear in the overview.
34. As the agent, I want to write each Segment with the Sponsor name, start and end time, whether it came from a Slot or from reading the Transcript, and the first line of the read, so that Patrick can jump to it and the overview can count it.
35. As the agent, I want to write the Sponsor name in a canonical form (the brand as it names itself, consistent casing), so that the same Sponsor across Videos groups together in the overview.
36. As the agent, I want to write an explicit "processed, no Sponsors found" Segments file for a Video with no reads, so that the overview can distinguish "done" from "not yet processed".
37. As Patrick, I want the agent pass to run on every Video, not only those without Slots, so that every Sponsor gets a name (SponsorBlock never names brands).
38. As Patrick, I want the Segments file to be plain, readable, and hand-editable, so that I can correct a mis-named Sponsor myself.
39. As Patrick, I want the agent to tell me which Videos it processed and how many Segments it wrote per Video, so that I can spot a Video that needs a second look.

### Sponsorship overview

40. As Patrick, I want `overview` to read every Segments file in the Corpus and write one overview file, so that I have a single place to look.
41. As Patrick, I want the overview grouped by Sponsor, listing each Channel it sponsors and the Videos the Sponsorship is evidenced by, so that I can answer "who does Sponsor X work with?".
42. As Patrick, I want the overview also grouped by Channel, listing each Sponsor that appears on it with the count of Segments, so that I can answer "who sponsors Channel Y?".
43. As Patrick, I want each Video reference in the overview to carry the Segment's start time, so that I can open the Video at the read when studying how it was written.
44. As Patrick, I want Sponsor names grouped case-insensitively and with surrounding whitespace ignored, so that a small inconsistency in the agent's naming doesn't split one Sponsor into two.
45. As Patrick, I want the overview to list Videos that have no Segments file yet, so that I know the agent-pass backlog.
46. As Patrick, I want the overview to list Videos recorded with no Transcript, so that I can decide whether to drop them or look for captions another way.
47. As Patrick, I want the overview written as Markdown, so that it reads well in the repo and in a diff.
48. As Patrick, I want `overview` to be idempotent and fast, so that I can regenerate it after every agent pass.
49. As Patrick, I want a Segments file that fails to parse to be reported by name and skipped, so that one hand-edit mistake doesn't blank the whole overview.

### Corpus and general

50. As Patrick, I want the Corpus laid out as one directory per Channel, holding one directory per Video named by its YouTube ID, so that I can browse a Channel's Videos directly and find a Video's files without the overview.
51. As Patrick, I want every Corpus file to be plain text (JSON, timestamped text, or Markdown) and committed to git, so that the Corpus is diffable and survives without any service.
52. As Patrick, I want the Corpus location to default to a directory in this repo and be overridable by a flag, so that tests and experiments can use a scratch Corpus.
53. As Patrick, I want the CLI to carry no API keys and make no LLM calls, so that running it costs nothing and needs no secrets (ADR-0001).
54. As Patrick, I want the CLI to build to a single binary with `go build`, so that there is nothing to install beyond Go.
55. As Patrick, I want clear `--help` output for every command, so that I don't have to reread the spec to use it.
56. As an agent implementing later features, I want the YouTube and SponsorBlock access behind one small interface each, so that an upstream endpoint change is a one-module fix.

## Implementation Decisions

### Shape

- One Go module, one binary, three subcommands: `fetch`, `sync`, `overview`. A `--corpus` flag (default: a `corpus` directory in the repo root) sets the Corpus root for every command.
- The CLI is deterministic. It never names a Sponsor, never calls an LLM, never holds an API key. All judgment is in the agent pass skill (ADR-0001).
- The repo keeps the Matt Pocock skill set under `.claude/skills`; the agent pass skill is added beside them as a project skill.

### Upstream access

- Two interfaces, one per upstream, and nothing else is abstracted:
  - **YouTube**: `ListNewestVideos(channelID)` returning Video metadata, and `FetchTranscript(videoID)` returning the English caption track as cues (start time, text) plus the Video's metadata, or a typed "no English track" result. The CLI renders the cues to the timestamped text file; the interface stays structured so the rendering is one place.
  - **SponsorBlock**: `FetchSlots(videoID)` returning the `sponsor`-category windows, or an empty list when SponsorBlock has no data (SponsorBlock answers 404 for unknown Videos; that is not an error).
- The real YouTube implementation is key-free: the Channel's public RSS/Atom video feed for `sync` (newest ~15 Videos per feed), and the public caption/timedtext route the YouTube web player uses for Transcripts. The timedtext route is undocumented and can shift; that risk is the reason it sits behind the interface and the reason the fake-backed tests don't depend on it.
- English track selection: prefer a manual `en` (or `en-*`) track, then an auto-generated `en` track, else "no transcript".
- Only SponsorBlock category `sponsor` is requested. Windows that overlap or touch are merged into one Slot; the merged Slot keeps the list of SponsorBlock segment IDs it came from.

### Corpus layout (described, not pathed)

- Followed Channels: one committed file at the Corpus root listing channel ID and display name per entry. Edited by hand. No `follow` command.
- One directory per Channel at the Corpus root, named by YouTube channel ID. A Channel directory is created the first time a Video from that Channel is fetched; a Channel can appear here without being a Followed Channel (fetched by hand) and vice versa (followed but nothing fetched yet).
- Inside each Channel directory, one directory per Video, named by YouTube video ID, holding:
  - a **video** metadata file: video ID, Channel ID, Channel name, title, publish date, fetch date, transcript status (`ok` / `none`) and track kind (`manual` / `auto`).
  - a **transcript** file: plain text, one caption cue per line, each line prefixed with its start time as `[mm:ss]` (or `[h:mm:ss]` past an hour). Not JSON: the CLI never reads it back, its only readers are the agent pass and Patrick, and timestamped text reads better and costs fewer tokens than cue objects. Absent when transcript status is `none`.
  - a **slots** file: list of Slots, each with start seconds, end seconds, source segment IDs. Present even when empty.
  - a **segments** file, written only by the agent pass: processed timestamp, and a list of Segments each with sponsor name, start seconds, end seconds, origin (`slot` or `transcript`), slot index when from a Slot, and the opening line of the read. An empty list with a processed timestamp means "processed, no Sponsors".
- The three files the CLI parses back (video, slots, segments) are JSON. The transcript is timestamped text. The overview is Markdown at the Corpus root.
- Write rules: `fetch` writes video/transcript/slots; with `--force` it rewrites them; it never creates, modifies, or deletes a segments file. `overview` writes only the overview file.

### Command behaviour

- Presence check: a Video is present when a directory with its ID exists under any Channel directory. The CLI finds it by a two-level scan of the Corpus; no index file is kept.
- `fetch <id-or-url> [--force]`: parse the ID from any YouTube URL form; if the Video is present and `--force` is absent, print "already present" and exit 0; otherwise call both upstreams (the YouTube transcript call also returns the Channel ID, which decides the Channel directory), write the files, print the summary line. Upstream failure: nothing partial is left behind (write into a temp dir and rename), stderr gets the reason, exit 1. A Video with no English track still gets its video and slots files written and exits 0.
- `sync`: for each Followed Channel, list newest Videos, and for each not already in the Corpus run the same fetch logic. Per-Channel and per-Video failures are collected, printed at the end, and do not stop the run. Exit 0 if at least one Channel was walked successfully, 1 if every Channel failed.
- `overview`: walk every Channel directory and every Video directory beneath it, parse the files, write the overview. Sections: **By Sponsor** (Sponsor → Channel → Videos with Segment start times), **By Channel** (Channel → Sponsor → Segment count), **Awaiting agent pass** (Videos with no segments file), **No transcript** (Videos with transcript status `none`). Sponsor grouping key is the trimmed, lower-cased name; the displayed name is the most frequent spelling. Unparseable files are reported on stderr and skipped. Exit 0 even with skipped files.

### Agent pass skill

- Invoked with a video ID or with no argument (meaning: every Video without a segments file).
- Reads the video, slots, and transcript files. With Slots: reads only the Transcript lines inside each Slot, widened by ~15 seconds on each side so a transition line isn't cut off. Without Slots: reads the whole Transcript.
- Decision rules written into the skill: a Segment is one Sponsor's paid read; back-to-back reads for different Sponsors in one Slot become separate Segments; two reads for the same Sponsor are two Segments; self-promotion and engagement reminders are excluded; the Sponsor name is the brand as it names itself (prefer the spelling on its domain when a URL is read out).
- Writes the segments file as described above, then prints a per-Video line: ID, Channel, number of Segments, Sponsor names.
- The skill never calls the CLI to fetch; it works only on Videos already in the Corpus.

## Testing Decisions

- **What a good test is here**: a test drives a command's entry function with a temporary Corpus directory and in-memory fakes of the two upstream interfaces, then asserts on the files that exist afterwards and on the text printed. Tests never inspect internal structs, never assert on which interface methods were called in which order, and never touch the network.
- **Seams** (confirmed with Patrick): exactly two, the YouTube interface and the SponsorBlock interface. The fakes are tiny maps keyed by video or channel ID that return canned Transcripts, feeds, Slots, "no English track", "SponsorBlock has no data", or an error.
- **Modules under test**: the `fetch`, `sync`, and `overview` command functions; URL-to-ID parsing; Slot merging; Sponsor-name grouping in the overview. The real upstream implementations get one opt-in live smoke test each, skipped unless an environment variable enables it, so CI stays offline.
- **Agent pass**: not covered by Go tests. Its contract is tested indirectly by `overview` tests that feed hand-written segments files (including the empty "processed, none found" shape and a malformed file).
- **Prior art**: none, the repo is greenfield. Use the standard library `testing` package with table tests; fixture JSON lives beside the tests.

## Out of Scope

- Our own speech-to-text or any non-YouTube Transcript source.
- The YouTube Data API, OAuth, or any keyed upstream.
- Non-English Transcripts.
- Any LLM call from inside the binary.
- A `follow` / `unfollow` command (edit the Followed Channels file by hand).
- Detecting or storing self-promotion, engagement reminders, or any SponsorBlock category other than `sponsor`.
- Backfilling a Channel beyond what its feed returns (~15 newest Videos).
- A web UI, dashboard, or search over the Corpus.
- Scheduling or automatic syncing.
- Dedup of re-uploaded Videos across IDs.
- Comparing Patrick's own channel; the Corpus is other creators' Channels only.

## Further Notes

- Respects ADR-0001 (judgment lives in the agent pass): the only place a Sponsor is named is the skill.
- The glossary's "avoid" terms (episode, captions, ad, deal, library...) should not appear in command names, flags, file names, or help text.
- Patrick prefers fewer, broader features; anything not listed above should be raised as a follow-up spec rather than slipped into a ticket.
- Next step: run `/to-tickets .scratch/corpus-v1/spec.md`.
