# 03: Sync Followed Channels

**What to build:** Patrick keeps a committed list of Followed Channels in the Corpus and runs `sync`. The CLI walks each Followed Channel's newest Videos and fetches every one not already in the Corpus, skipping and reporting failures instead of aborting, and ends with a summary. Running it twice in a row fetches nothing the second time. No API key is needed.

Spec: `.scratch/corpus-v1/spec.md` (section "Following Channels and syncing", "Command behaviour", "Upstream access"). No `follow` command: the list is edited by hand.

**Blocked by:** 01 (Fetch a Video's Transcript into the Corpus)

**Status:** done

- [x] A Followed Channels file lives at the Corpus root, listing channel ID and display name per entry, in a format Patrick can edit by hand. The CLI reads it; nothing writes it.
- [x] `ListNewestVideos(channelID)` on the real YouTube implementation reads the Channel's public RSS/Atom video feed (key-free) and returns the Videos it lists with their IDs, titles, and publish dates.
- [x] `sync` walks every Followed Channel, lists its newest Videos, and runs the same fetch logic as `fetch` for each Video not already present under any Channel directory. Videos already present are counted as skipped and not refetched.
- [x] A Channel whose feed can't be read is named in the summary and the run continues with the next Channel. A Video whose fetch fails is named in the summary and the run continues.
- [x] The run ends with a summary: Channels walked, Videos fetched, Videos skipped as already present, Videos failed, plus the names of failed Channels and Videos.
- [x] Exit 0 if at least one Channel was walked successfully; exit 1 if every Channel failed. A missing or empty Followed Channels file is reported clearly and exits 1.
- [x] Fake-backed tests cover: two Channels with new Videos get fetched into the right Channel directories; second run skips everything; one Channel's feed error doesn't stop the other; one Video's transcript error doesn't stop the rest; all Channels failing exits 1; a Video that is present under a different Channel directory than expected is still treated as present.

## Comments

- Built test-first on branch `ticket/03-sync-followed-channels`. `Sync` lives in `internal/cli/sync.go` and reuses `fetchVideo` untouched; the Followed Channels reader is `internal/corpus/followed.go` (`<corpus>/followed-channels.json`, read-only).
- The real `ListNewestVideos` reads the Atom feed `https://www.youtube.com/feeds/videos.xml?channel_id=...`, covered by an httptest fixture test plus a `YT_LIVE=1` smoke test; the sandbox blocks YouTube so the live test was not run.
- Summary lines: `channels walked / videos fetched / videos skipped (already present) / videos failed`, then `failed channels:` and `failed videos:` when any. Per-failure detail goes to stderr as it happens.
