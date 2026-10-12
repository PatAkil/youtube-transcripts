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
go test ./...     # offline; set YT_LIVE=1 to also run the live YouTube smoke test
```

## Commands

```sh
youtube-transcripts [--corpus DIR] <command> [arguments]
```

| Command    | What it does                                                                        |
| ---------- | ----------------------------------------------------------------------------------- |
| `fetch`    | Store one Video's Transcript and metadata in the Corpus. Takes a video ID or any YouTube URL (watch, youtu.be, shorts, embed). `--force` refetches a Video already present. |
| `sync`     | Fetch new Videos from every Followed Channel. _Coming._                              |
| `overview` | Write the Sponsorship overview from the Segments files. _Coming._                   |

The global `--corpus DIR` flag sets the Corpus root. It defaults to `corpus`,
relative to the working directory, so running from the repo root uses
`./corpus`. Every command has `--help`.

```sh
./youtube-transcripts fetch https://www.youtube.com/watch?v=dQw4w9WgXcQ
./youtube-transcripts --corpus /tmp/scratch fetch dQw4w9WgXcQ --force
```

## Corpus layout

One directory per Channel (named by YouTube channel ID), holding one directory
per Video (named by YouTube video ID). `fetch` writes:

```
corpus/
  <channelID>/
    <videoID>/
      video.json        # video_id, channel_id, channel_name, title, published,
                        # fetched, transcript ("ok" | "none"), track ("manual" | "auto" | "")
      transcript.txt    # one cue per line: "[mm:ss] text", "[h:mm:ss]" from one hour on;
                        # absent when transcript is "none"
```

A Video with no English track is still recorded (`"transcript": "none"`) so it
is not retried by accident. Fetching a Video that is already present is a no-op
unless `--force` is given, and a refetch never touches a `segments.json` written
by the agent pass. All files are plain text and meant to be committed.
