# 13. Zero-Knowledge Seekable Stream Encryption for File Parts

We decided to use a seekable stream cipher (AES-CTR with key and IV derived deterministically from the instance secret key and file ID) for Zero-Knowledge Part Encryption instead of chunked AEAD (such as AES-GCM).

Telegram MTProto's `upload.saveBigFilePart` strictly enforces part sizes that are multiples of 1024 bytes (standardized at 512 KB) for all parts except the terminal part. Chunked AEAD introduces per-part nonce and authentication tag expansion (e.g., 28 bytes), which distorts MTProto chunk boundaries and complicates HTTP 206 Range Request byte offset mapping. A stream cipher maintains exact 1-to-1 byte parity, allowing instantaneous O(1) seekable video and media streaming without temporary buffering or complex chunk offset re-indexing.
