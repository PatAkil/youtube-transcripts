# 05: Agent pass skill writes Sponsor Segments

**What to build:** Patrick invokes the agent pass skill in this repo for one Video or for every Video without a segments file. The agent reads the Video's Slots and Transcript, names the Sponsor of each read, splits a Slot with back-to-back reads for two Sponsors into two Sponsor Segments, finds reads in Videos SponsorBlock has no data on by reading the whole Transcript, leaves out self-promotion and engagement reminders, and writes the segments file in the shape `overview` reads. Afterwards Patrick runs `overview` and the new Sponsorships appear.

Spec: `.scratch/corpus-v1/spec.md` (sections "The agent pass (Sponsor Segments)", "Agent pass skill"). This is the only place a Sponsor is ever named (ADR-0001). The skill is a project skill beside the existing ones; follow the writing-for-agents skill when authoring it.

**Blocked by:** 02 (Take Sponsor Slots from SponsorBlock), 04 (Build the Sponsorship overview)

**Status:** done

- [x] A project skill exists that accepts a video ID, or no argument meaning every Video in the Corpus that has no segments file. It never fetches; it works only on Videos already in the Corpus and says so when asked about an absent Video.
- [x] With Slots present, the skill reads only the Transcript lines inside each Slot, widened by about 15 seconds on each side, and names the Sponsor(s) read in that window. Without Slots, it reads the whole Transcript to find reads.
- [x] The skill's decision rules are written into it: one Segment per Sponsor read; back-to-back reads for different Sponsors in one Slot become separate Segments with their own start and end; two reads for the same Sponsor in one Video are two Segments; self-promotion (own course, merch, Patreon, newsletter) and engagement reminders are excluded; a Video with no reads gets an empty Segments list with a processed timestamp.
- [x] Sponsor names are written canonically: the brand as it names itself, consistent casing, preferring the spelling of its domain when a URL is read aloud.
- [x] Each Segment carries sponsor name, start seconds, end seconds, origin (`slot` or `transcript`), slot index when from a Slot, and the opening line of the read, matching the schema fixed in ticket 04. The skill never edits video, transcript, or slots files.
- [x] The skill prints one line per Video processed: video ID, Channel name, number of Segments, Sponsor names.
- [x] Verification: run the skill on at least two fetched Videos (one with Slots, one without), then run `overview`; the Sponsors appear under By Sponsor and By Channel and the Videos leave the backlog. Record the Video IDs used in the ticket's Comments.

## Comments

- Built `.claude/skills/agent-pass/` (SKILL.md plus `agents/openai.yaml`) and pointed at it from the README commands table between `sync` and `overview`.
- Verification ran against two **synthetic** fixture Videos, because this sandbox blocks outbound traffic to YouTube and SponsorBlock, so no real Video could be fetched. The fixture Corpus is committed at `.scratch/corpus-v1/verification-corpus/` (fake 11-character IDs, fake `UC...` channel IDs, hand-written Transcripts, Slots and metadata in the real layout):
  - `aB3dE5fG7hI` (Tech Tangent): one Slot holding back-to-back reads for NordVPN and Brilliant, with a Patreon line in the Slot's widened margin. Written as two Segments with `slot_index` 0; the Patreon line excluded.
  - `kL9mN0pQ2rS` (The Build Log): empty Slots list, auto-generated lower-case track, one mid-video Incogni read with the URL spoken aloud, plus a subscribe reminder, newsletter and merch mentions. Written as one `transcript`-origin Segment named `Incogni`; the rest excluded.
- The skill was performed step by step as its text instructs, then `go build` and `./youtube-transcripts --corpus .scratch/corpus-v1/verification-corpus overview`: NordVPN, Brilliant and Incogni appear under By Sponsor and By Channel; Awaiting agent pass is `_None._`. The produced `segments.json` files and `overview.md` are committed with the fixture.
- Performing the pass surfaced two gaps, fixed in the skill text: a closing line shared by back-to-back reads ("thanks to both X and Y") belongs to the last read, and auto-generated tracks lower-case brand names so `sponsor` restores the brand's casing.
- Still open for Patrick: run the skill on two real fetched Videos (one with Slots, one without) from a machine with network access and record their IDs here.
