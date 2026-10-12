# 03: Sync Followed Channels

**What to build:** Patrick keeps a committed list of Followed Channels in the Corpus and runs `sync`. The CLI walks each Followed Channel's newest Videos and fetches every one not already in the Corpus, skipping and reporting failures instead of aborting, and ends with a summary. Running it twice in a row fetches nothing the second time. No API key is needed.

Spec: `.scratch/corpus-v1/spec.md` (section "Following Channels and syncing", "Command behaviour", "Upstream access"). No `follow` command: the list is edited by hand.

**Blocked by:** 01 (Fetch a Video's Transcript into the Corpus)

**Status:** ready-for-agent

- [ ] A Followed Channels file lives at the Corpus root, listing channel ID and display name per entry, in a format Patrick can edit by hand. The CLI reads it; nothing writes it.
- [ ] `ListNewestVideos(channelID)` on the real YouTube implementation reads the Channel's public RSS/Atom video feed (key-free) and returns the Videos it lists with their IDs, titles, and publish dates.
- [ ] `sync` walks every Followed Channel, lists its newest Videos, and runs the same fetch logic as `fetch` for each Video not already present under any Channel directory. Videos already present are counted as skipped and not refetched.
- [ ] A Channel whose feed can't be read is named in the summary and the run continues with the next Channel. A Video whose fetch fails is named in the summary and the run continues.
- [ ] The run ends with a summary: Channels walked, Videos fetched, Videos skipped as already present, Videos failed, plus the names of failed Channels and Videos.
- [ ] Exit 0 if at least one Channel was walked successfully; exit 1 if every Channel failed. A missing or empty Followed Channels file is reported clearly and exits 1.
- [ ] Fake-backed tests cover: two Channels with new Videos get fetched into the right Channel directories; second run skips everything; one Channel's feed error doesn't stop the other; one Video's transcript error doesn't stop the rest; all Channels failing exits 1; a Video that is present under a different Channel directory than expected is still treated as present.
