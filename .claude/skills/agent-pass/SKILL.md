---
name: agent-pass
description: "Perform the agent pass on the Corpus: read a Video's Sponsor Slots and Transcript, name the Sponsor of each read, and write its segments.json. Use when asked to run the agent pass, name the sponsors, write segments, or process the corpus."
---

The user has asked for the **agent pass**: the one place a Sponsor is ever named (ADR-0001, `docs/adr/0001-judgment-lives-in-the-agent-pass.md`). You read what `fetch` stored and write one `segments.json` per Video. Vocabulary is in `GLOSSARY.md`.

Argument: a video ID, or nothing, meaning every Video in the Corpus without a `segments.json`.

## Steps

1. **Locate the Videos.** The Corpus root is `corpus/` at the repo root unless the user names another directory (the CLI's `--corpus DIR`). A Video lives at `<corpus>/<channelID>/<videoID>/`, so scan two levels. With a video ID, process that one directory; when no directory carries that ID, stop and tell the user the Video is absent from the Corpus and that `youtube-transcripts fetch <id>` stores it (the pass never fetches). With no argument, take every Video directory that has a `video.json` and no `segments.json`. Leave out Videos whose `video.json` says `"transcript": "none"`: there is nothing to read, and `overview` lists them under No transcript. _Done when_ you hold the list of Video directories to process, each with `video.json`, `slots.json` and `transcript.txt`.

2. **Read the Video.** Read `video.json` (for `channel_name`) and `slots.json`. With Slots, read only the Transcript lines whose timestamp falls inside each Slot widened by 15 seconds on each side, so the transition in and out is in view. Without Slots (`"slots": []`), read the whole `transcript.txt`. A Transcript line is `[mm:ss] text` or `[h:mm:ss] text`, the timestamp being the line's start in the Video. _Done when_ every Slot window, or the whole Transcript, has been read.

3. **Decide the Segments** with the decision rules below, Slot by Slot (or across the whole Transcript when there are none). _Done when_ every read inside the windows is either a Segment or named self-promotion or an engagement reminder, and every Segment has a canonical Sponsor name, start, end, origin and opening line.

4. **Write `segments.json`** into the Video directory, in the shape fixed by `docs/segments-file.md` (`processed` is now, RFC 3339 UTC; two-space indent, trailing newline, Segments in Video order). A Video with no reads gets `"segments": []`. This is the only file you write: `video.json`, `transcript.txt` and `slots.json` are the CLI's. _Done when_ the file is on disk and matches the schema.

5. **Report.** Print one line per Video processed:

   ```
   <videoID>  <channelName>  segments=<n>  <Sponsor>, <Sponsor>
   ```

   (`segments=0` with no names when none were found.) Then suggest `youtube-transcripts overview` so the new Sponsorships appear in `overview.md`. _Done when_ every Video from step 1 has its line.

## Decision rules

- A Segment is one Sponsor's **paid read**: the Channel is paid by a brand to promote it. One Segment per read.
- Back-to-back reads for **different Sponsors** in one Slot are separate Segments, each with its own start and end, sharing the Slot's `slot_index`.
- Two reads for the **same Sponsor** in one Video (an intro read and a mid-roll read) are two Segments.
- **Self-promotion** is not a Segment: the Channel's own course, merch, Patreon, newsletter, Discord, second channel, live show. **Engagement reminders** (like, subscribe, comment, notification bell) are not Segments either. A Slot holding only these yields no Segment.
- A read found by reading the whole Transcript has `"origin": "transcript"` and no `slot_index`; a read found inside a Slot has `"origin": "slot"` and that Slot's index.
- A read that begins inside a Slot's widened margin, before or after the Slot proper, still belongs to that Slot.
- No reads at all: `"segments": []` with the `processed` timestamp. The overview then counts the Video as done.

## Timing a Segment

- `start` is the timestamp of the read's opening line, in seconds (`[mm:ss]` is `mm*60+ss`; `[h:mm:ss]` adds `h*3600`).
- `end` is the timestamp of the first Transcript line after the read (the line where the content, or the next Sponsor's read, resumes). When the read's last line is the Slot's last line, use the Slot's `end`.
- A closing line shared by back-to-back reads ("thanks to both X and Y") belongs to the last read; the earlier read ends where the next Sponsor is introduced.
- `opening_line` is the full text of the read's first Transcript line, without the timestamp. That is the first line that names the Sponsor ("This video is sponsored by NordVPN."); a lead-in such as "a quick word about today's sponsor" stays outside the Segment.

## Naming a Sponsor

- Write the brand as it names itself, with its own casing: `NordVPN`, `Squarespace`, `Brilliant`, `Ground News`, `iFixit`. An auto-generated track (`"track": "auto"`) lower-cases everything; restore the brand's casing in `sponsor` and keep `opening_line` as the Transcript has it.
- When a URL is read aloud ("go to incogni dot com slash ..."), prefer the spelling of the domain's brand (`Incogni`); when the brand and domain differ in form (`squarespace.com` for Squarespace), the brand name wins.
- Reuse the spelling already in another Video's `segments.json` for the same Sponsor; `grep -rh '"sponsor"' <corpus>` shows the names in use. The overview groups names case-insensitively, but one spelling keeps the displayed name right.
- A product line read under a parent brand takes the name the read itself uses ("the Opera GX browser" is `Opera GX`).
