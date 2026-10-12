# 02: Take Sponsor Slots from SponsorBlock

**What to build:** When Patrick fetches a Video, the CLI also asks SponsorBlock for the Video's sponsor windows and stores them as Sponsor Slots in the Video directory, so the agent pass can jump straight to the relevant part of the Transcript. Only SponsorBlock's `sponsor` category counts; self-promotion and engagement reminders are not Slots. A Video SponsorBlock knows nothing about gets an empty Slots list, not an error.

Spec: `.scratch/corpus-v1/spec.md` (stories on Slots under "Fetching one Video", "Upstream access", "Corpus layout"). Glossary: a Sponsor Slot is a contiguous window; avoid "ad break", "sponsor block" as a domain term.

**Blocked by:** 01 (Fetch a Video's Transcript into the Corpus)

**Status:** done

- [x] SponsorBlock access sits behind a single interface with `FetchSlots(videoID)` returning the `sponsor`-category windows, or an empty list when SponsorBlock answers 404 (not an error).
- [x] The real implementation calls SponsorBlock's public skip-segments endpoint with only the `sponsor` category and no key.
- [x] Windows that overlap or touch are merged into one Slot. Each Slot records start seconds, end seconds, and the SponsorBlock segment IDs it was merged from.
- [x] `fetch` writes a JSON slots file in the Video directory on every successful fetch, including when the list is empty, and including when the Video has no English track.
- [x] `--force` rewrites the slots file; the segments file is still never touched.
- [x] A SponsorBlock failure other than "no data" is reported on stderr and fails the fetch with exit 1, leaving nothing partial behind (atomic write covers both files).
- [x] The success summary line now also prints the number of Slots.
- [x] Fake-backed tests cover: Slots written, empty list on no data, merge of overlapping windows, merge of touching windows, non-overlapping windows stay separate, SponsorBlock error fails the fetch, `--force` rewrites slots but leaves an existing segments file byte-identical.
- [x] One live smoke test against real SponsorBlock exists and is skipped unless an environment variable enables it.

## Comments

- Done on branch `ticket/02-sponsor-slots` (commits 0567895, 28d3aca, 6d19365): `internal/sponsorblock` (Client, HTTPClient, Fake, httptest-backed tests), `internal/corpus/slots.go` (Slot, Slots, SlotsFile, MergeWindows), `fetchVideo` now calls SponsorBlock after YouTube and writes `slots.json`; `Deps.SponsorBlock` wired in `main.go`; README updated. 11 new test functions, all offline.
- Nothing deferred. The `fetch --help` text was left as ticket 01 wrote it (it does not mention Slots) to keep `fetch.go` edits inside `fetchVideo`; a one-line help touch-up can ride along with ticket 03 or 04.
- Live smoke test: `SPONSORBLOCK_LIVE=1 go test ./internal/sponsorblock -run TestLiveFetchSlots -v` (needs network to sponsor.ajay.app; blocked in the sandbox, so it is unverified there).
