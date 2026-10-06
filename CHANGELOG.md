# Changelog

## 4.20.0

- Double-buffers the Markdown Preview pane so RichEdit text and custom code-block cards are composed off-screen and presented as one frame.
- Suppresses Preview background erasing to prevent white/dark flashes between scroll frames.
- Removes the redundant full-preview invalidation from every smooth-scroll tick; RichEdit now invalidates only the scrolled region itself.
- Keeps the existing smooth wheel behavior, custom rounded fenced-code blocks, and v4.19 scroll-position preservation.

## 4.19.0

- Live Split/Preview rendering now preserves the preview pane's exact pixel scroll position while you type, instead of jumping back to the top on every refresh.
- Fixed deleted files being silently recreated by autosave while their tabs were still open.
- External deletion now suspends disk autosave for that tab while preserving the in-memory/recovery buffer.
- Ctrl+S on an externally deleted note explicitly asks whether to restore the file.
- Ctrl+Shift+S remains the safe way to keep the buffer at a new location.
- Restoring a deleted file from Explorer/Recycling automatically reconnects the open tab to the disk file.
- The missing-on-disk state is persisted across restarts, so reopening Typora cannot resurrect a deleted note.
