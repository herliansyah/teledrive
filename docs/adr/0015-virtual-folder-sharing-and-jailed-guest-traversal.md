# 15. Virtual Folder Sharing and Jailed Guest Traversal

We decided to extend public share links to support entire Virtual Folders using SQLite Common Table Expressions (CTE) for strict guest boundary jailing.

Rather than flattening shared folders into temporary archives or granting uncontained directory traversal, guests navigate subfolders and stream or download nested files dynamically. Every guest subfolder navigation and download request executes a recursive CTE query verifying that the target item descends strictly from the shared folder root. This prevents path traversal and unauthorized item access across the virtual drive without duplicating storage references or state.
