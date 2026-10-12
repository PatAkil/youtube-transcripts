# youtube-transcripts

A Corpus of English Transcripts from other creators' YouTube Videos, collected so
the Sponsor Segments in them can be studied. The Go CLI here fetches and
maintains the Corpus; the judgment work (naming Sponsors) is done by an agent
skill, not by the binary (see `docs/adr/0001-judgment-lives-in-the-agent-pass.md`).
Vocabulary is in `GLOSSARY.md`.

## Build

Requires Go 1.24 or newer and nothing else; the CLI uses only the standard
library and needs no API key.

```sh
go build          # produces ./youtube-transcripts
go test ./...     # offline; set YT_LIVE=1 and/or SPONSORBLOCK_LIVE=1 to also run
                  # the live YouTube and SponsorBlock smoke tests
```

## Commands

```sh
youtube-transcripts [--corpus DIR] <command> [arguments]
```

| Command    | What it does                                                                        |
| ---------- | ----------------------------------------------------------------------------------- |
| `fetch`    | Store one Video's Transcript, metadata and Sponsor Slots in the Corpus. Takes a video ID or any YouTube URL (watch, youtu.be, shorts, embed). `--force` refetches a Video already present. |
| `sync`     | Fetch new Videos from every Followed Channel listed in `corpus/followed-channels.json`. Videos already present are skipped; a failing Channel or Video is named in the closing summary and the run continues. |
| `overview` | Write the Sponsorship overview to `<corpus>/overview.md` from every `segments.json`: By Sponsor, By Channel, Awaiting agent pass, No transcript. Idempotent. |

The global `--corpus DIR` flag sets the Corpus root. It defaults to `corpus`,
relative to the working directory, so running from the repo root uses
`./corpus`. Every command has `--help`.

```sh
./youtube-transcripts fetch https://www.youtube.com/watch?v=dQw4w9WgXcQ
./youtube-transcripts --corpus /tmp/scratch fetch dQw4w9WgXcQ --force
./youtube-transcripts sync
./youtube-transcripts overview
```

`sync` exits 0 when at least one Followed Channel was walked and 1 when every
Channel failed or there are no Followed Channels. It needs no API key: each
Channel's public feed lists its newest ~15 Videos, so a Channel is backfilled
only that far.

## Corpus layout

The Followed Channels file and the overview sit at the Corpus root, then one
directory per Channel (named by YouTube channel ID), holding one directory per
Video (named by YouTube video ID):

```
corpus/
  followed-channels.json   # the Followed Channels, edited by hand and read by sync:
                           # [{"channel_id": "UC...", "name": "Display Name"}, ...]
  overview.md              # the Sponsorship overview; written by overview only
  <channelID>/
    <videoID>/
      video.json        # video_id, channel_id, channel_name, title, published,
                        # fetched, transcript ("ok" | "none"), track ("manual" | "auto" | "")
      transcript.txt    # one cue per line: "[mm:ss] text", "[h:mm:ss]" from one hour on;
                        # absent when transcript is "none"
      slots.json        # Sponsor Slots from SponsorBlock: {"slots": [{"start", "end",
                        # "source_ids": [...]}]}; present even when the list is empty
      segments.json     # the Video's Sponsor Segments, written by the agent pass only;
                        # schema in docs/segments-file.md
```

`fetch` and `sync` write `video.json`, `transcript.txt` and `slots.json`. The
agent pass writes `segments.json` ([schema](docs/segments-file.md)); a Video
without one is the agent-pass backlog that `overview` lists.

Slots come from SponsorBlock's public skip-segments endpoint, `sponsor`
category only (self-promotion and engagement reminders are not Slots), with no
key. Windows that overlap or touch are merged into one Slot that keeps the IDs
of the SponsorBlock segments it came from. A Video SponsorBlock has no data on
gets an empty list; any other SponsorBlock failure fails the fetch and leaves
nothing partial behind.

A Video with no English track is still recorded (`"transcript": "none"`) so it
is not retried by accident. Fetching a Video that is already present is a no-op
unless `--force` is given, and a refetch never touches a `segments.json` written
by the agent pass. Presence is decided by video ID under any Channel directory,
so `sync` never refetches a Video that was fetched by hand. All files are plain
text and meant to be committed; nothing writes `followed-channels.json` but you.
