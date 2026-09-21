# 16. Recursive Folder Upload and In-App Self-Update

We decided to support recursive directory tree uploads with explicit conflict resolution strategies, and embed self-updating capabilities directly into the single binary.

Rather than requiring users to manually reconstruct folder hierarchies or zip archives prior to upload, TeleDrive traverses directory trees via the HTML5 FileSystem API in the browser or `filepath.WalkDir` in the CLI, mapping nested directories to SQLite Virtual Folders on the fly while streaming files sequentially through MTProto. For name collisions, interactive policies (Replace, Keep Both with auto-increment, or Skip) allow granular user intent without data loss.

For lifecycle management, TeleDrive integrates an automated release checker against official GitHub Releases. To preserve the single-binary zero-dependency ethos, updates download the matching OS/architecture artifact, perform an atomic binary replacement on disk, and initiate an in-place graceful restart without wiping SQLite metadata or session keys.
