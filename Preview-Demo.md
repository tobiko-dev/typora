# Native Typora v4

The editor shell is **native Windows code** rather than a browser wrapper.

## Saving

- New tabs are saved to your chosen notes folder.
- `Ctrl+S` flushes immediately.
- `Ctrl+Shift+S` opens **Save As**.
- Recovery buffers keep the latest session text separately.

> Close the app and your tabs should return next launch.

## Task list

- [x] Native shell
- [x] Multiple tabs
- [x] Persistent session buffers
- [x] Automatic note naming
- [x] Rounded native fenced-code cards
- [x] Flicker-reduced preview scrolling

| Feature | Behavior |
| --- | --- |
| New tab | Starts at `Untitled 01` or next free number |
| Rename | Inline on creation / F2 |
| Recovery | ~100 ms idle |
| Disk save | ~600 ms idle |
