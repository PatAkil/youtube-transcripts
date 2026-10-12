# Judgment lives in the agent pass, not the CLI

The Go CLI is deterministic: it fetches Transcripts, takes Sponsor Slots from SponsorBlock, syncs Followed Channels, and builds the Sponsorship overview. Everything that needs judgment (naming the Sponsor, splitting a Slot with two reads into two Sponsor Segments, finding reads in Videos SponsorBlock has no data on) is done by an agent running a skill in this repo, which writes the Segments files the CLI later reads. We chose this over calling an LLM API from inside the binary so the CLI carries no API keys or per-run cost and an agent is the primary caller anyway.

## Consequences

- Running the CLI by hand yields Transcripts and Slots but no Segments until an agent pass runs.
- SponsorBlock never names brands, so the agent pass runs on every Video, not only as a fallback. With a Slot it reads only the Slot text; without one it reads the whole Transcript.
