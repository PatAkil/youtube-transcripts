# 01: Fetch a Video's Transcript into the Corpus

**What to build:** Patrick runs `fetch` with a YouTube video ID or any YouTube URL form and the Video's English Transcript lands in the Corpus as readable timestamped text, next to a video metadata file, inside a Channel directory. Running it again is a no-op unless `--force` is passed. A Video with no English caption track is still recorded, with a `none` transcript status, so it isn't retried by accident. The CLI needs no API key.

Spec: `.scratch/corpus-v1/spec.md` (sections "Fetching one Video", "Corpus layout", "Command behaviour", "Upstream access"). Respect ADR-0001: no judgment, no LLM calls, no keys in the binary. Use glossary terms (Video, Channel, Transcript, Corpus) in names and help text; avoid "captions", "episode", "library".

**Blocked by:** None (can start immediately)

**Status:** ready-for-agent

- [ ] A Go module builds to a single binary with `go build`; `fetch` and a global `--corpus` flag (default: a `corpus` directory in the repo root) exist with `--help` text.
- [ ] `fetch` accepts a bare video ID and the watch, youtu.be, shorts, and embed URL forms, including extra query parameters, and extracts the same ID from each.
- [ ] YouTube access sits behind a single interface with `ListNewestVideos(channelID)` and `FetchTranscript(videoID)`; `FetchTranscript` returns the Video's metadata and the English track as structured cues (start time, text) or a typed "no English track" result. `ListNewestVideos` may be left unimplemented in the real client for this ticket (ticket 03 fills it in).
- [ ] The real YouTube implementation is key-free: it uses the public caption route the YouTube web player uses. It prefers a manual `en`/`en-*` track, then an auto-generated `en` track.
- [ ] On success the CLI writes, under `<corpus>/<channelID>/<videoID>/`, a JSON video metadata file (video ID, Channel ID, Channel name, title, publish date, fetch date, transcript status `ok`/`none`, track kind `manual`/`auto`) and a plain-text transcript file with one cue per line prefixed `[mm:ss]` (or `[h:mm:ss]` past an hour).
- [ ] With no English track, the video metadata file is written with transcript status `none`, no transcript file is written, and the command exits 0.
- [ ] If a directory with the Video's ID already exists under any Channel directory and `--force` is absent, the CLI prints "already present" and exits 0 without calling YouTube. With `--force` it refetches and rewrites the video and transcript files. It never creates, modifies, or deletes a segments file if one exists.
- [ ] Writes are atomic: files are written to a temporary directory and renamed into place, so an upstream failure leaves nothing partial behind.
- [ ] On upstream failure the reason goes to stderr and the exit code is 1.
- [ ] The success summary line prints the video ID, Channel name, title, and number of Transcript lines.
- [ ] Tests drive the `fetch` entry function with a temporary Corpus directory and an in-memory fake of the YouTube interface, asserting only on files written and text printed: happy path (manual track), auto track fallback, no English track, already present, `--force`, upstream error leaves no partial directory, URL parsing table.
- [ ] One live smoke test against real YouTube exists and is skipped unless an environment variable enables it.
