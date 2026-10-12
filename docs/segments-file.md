# The segments file (`segments.json`)

`<corpus>/<channelID>/<videoID>/segments.json` records the agent pass's verdict
on one Video: which Sponsor Segments it holds. It is written only by the agent
pass (ADR-0001: the CLI never names a Sponsor). `overview` reads it to build
`overview.md`, and `fetch --force` carries it over untouched. The Go types and
parser are in `internal/corpus/segments.go` (`corpus.ReadSegments`).

## Example: two Segments

```json
{
  "processed": "2026-10-12T09:30:00Z",
  "segments": [
    {
      "sponsor": "NordVPN",
      "start": 12.5,
      "end": 60.0,
      "origin": "slot",
      "slot_index": 0,
      "opening_line": "This video is sponsored by NordVPN."
    },
    {
      "sponsor": "Squarespace",
      "start": 1830.0,
      "end": 1895.0,
      "origin": "transcript",
      "opening_line": "Before we go on, a word about Squarespace."
    }
  ]
}
```

## Example: processed, no Sponsors found

```json
{
  "processed": "2026-10-12T09:30:00Z",
  "segments": []
}
```

An empty list with a `processed` timestamp means the agent pass ran and found
no Sponsor Segments. The overview then lists the Video nowhere: not under any
Sponsor and not in the agent-pass backlog. A Video with no segments file at all
is the backlog.

## Fields

| Field          | Type                | Meaning                                                                                          |
| -------------- | ------------------- | ------------------------------------------------------------------------------------------------ |
| `processed`    | RFC 3339 timestamp  | When the agent pass wrote the file. Required.                                                    |
| `segments`     | list                | One entry per Sponsor Segment, in Video order. May be empty.                                     |
| `sponsor`      | string              | The Sponsor's canonical name: the spelling the Sponsor itself uses, consistent casing. Required.         |
| `start`, `end` | number, seconds     | Where the read begins and ends in the Video. `start >= 0`, `end >= start`.                       |
| `origin`       | `slot`/`transcript` | `slot`: found inside a Sponsor Slot from SponsorBlock. `transcript`: found by reading the Transcript. |
| `slot_index`   | integer             | Index into `slots.json` when `origin` is `slot`. Omitted when `origin` is `transcript`.          |
| `opening_line` | string              | The first Transcript line of the read, so a reader can find it without the timestamp.            |

## Rules

- `start` and `end` are seconds from the start of the Video, as in `slots.json`
  (the overview renders them as `[mm:ss]`).
- Two reads for the same Sponsor in one Video are two Segments; a Slot holding
  back-to-back reads for two Sponsors yields two Segments with the same
  `slot_index`.
- Self-promotion and engagement reminders are not Segments.
- The Sponsor name is grouped case-insensitively and trimmed by the overview,
  but write it canonically anyway so the displayed spelling is right.
- The file is hand-editable. `overview` names a file that fails to parse on
  stderr and skips that Video (unknown `origin`, missing `processed`, negative
  or inverted times, a blank `sponsor`, or `slot` without `slot_index`).
- Written only by the agent pass. `fetch` and `sync` never create, change or
  delete it.
