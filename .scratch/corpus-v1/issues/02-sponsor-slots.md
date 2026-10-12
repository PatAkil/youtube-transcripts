# 02: Take Sponsor Slots from SponsorBlock

**What to build:** When Patrick fetches a Video, the CLI also asks SponsorBlock for the Video's sponsor windows and stores them as Sponsor Slots in the Video directory, so the agent pass can jump straight to the relevant part of the Transcript. Only SponsorBlock's `sponsor` category counts; self-promotion and engagement reminders are not Slots. A Video SponsorBlock knows nothing about gets an empty Slots list, not an error.

Spec: `.scratch/corpus-v1/spec.md` (stories on Slots under "Fetching one Video", "Upstream access", "Corpus layout"). Glossary: a Sponsor Slot is a contiguous window; avoid "ad break", "sponsor block" as a domain term.

**Blocked by:** 01 (Fetch a Video's Transcript into the Corpus)

**Status:** ready-for-agent

- [ ] SponsorBlock access sits behind a single interface with `FetchSlots(videoID)` returning the `sponsor`-category windows, or an empty list when SponsorBlock answers 404 (not an error).
- [ ] The real implementation calls SponsorBlock's public skip-segments endpoint with only the `sponsor` category and no key.
- [ ] Windows that overlap or touch are merged into one Slot. Each Slot records start seconds, end seconds, and the SponsorBlock segment IDs it was merged from.
- [ ] `fetch` writes a JSON slots file in the Video directory on every successful fetch, including when the list is empty, and including when the Video has no English track.
- [ ] `--force` rewrites the slots file; the segments file is still never touched.
- [ ] A SponsorBlock failure other than "no data" is reported on stderr and fails the fetch with exit 1, leaving nothing partial behind (atomic write covers both files).
- [ ] The success summary line now also prints the number of Slots.
- [ ] Fake-backed tests cover: Slots written, empty list on no data, merge of overlapping windows, merge of touching windows, non-overlapping windows stay separate, SponsorBlock error fails the fetch, `--force` rewrites slots but leaves an existing segments file byte-identical.
- [ ] One live smoke test against real SponsorBlock exists and is skipped unless an environment variable enables it.
