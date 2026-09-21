# 14. Embedded WebDAV Gateway on Single Port

We decided to embed a standard WebDAV (RFC 4918) handler directly into the existing `net/http` router under the path prefix `/webdav` using `golang.org/x/net/webdav`.

This provides seamless OS-level file mounting (Windows Explorer, macOS Finder, Linux, and rclone) without introducing external sync daemons or opening secondary network ports. Virtual folder hierarchy and file operations are bridged directly to SQLite metadata and Telegram MTProto streaming.
