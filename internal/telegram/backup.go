package telegram

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gotd/td/tg"
	"teledrive/internal/db"
)

// SnapshotInfo represents metadata of a point-in-time database snapshot.
type SnapshotInfo struct {
	MessageID int       `json:"message_id"`
	FileName  string    `json:"file_name"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
	IsPinned  bool      `json:"is_pinned"`
}

// UploadSnapshot uploads a compressed SQLite database snapshot to the Storage Channel,
// pins it, and automatically prunes older snapshots past the retention limit (5).
func (m *ClientManager) UploadSnapshot(ctx context.Context, gzPath string, limiter *SafeLimiter) (int, error) {
	fileInfo, err := os.Stat(gzPath)
	if err != nil {
		return 0, fmt.Errorf("stat snapshot: %w", err)
	}

	f, err := os.Open(gzPath)
	if err != nil {
		return 0, fmt.Errorf("open snapshot: %w", err)
	}
	defer f.Close()

	fileName := filepath.Base(gzPath)
	msgID, _, _, _, err := m.UploadFromReader(ctx, f, fileInfo.Size(), fileName, "application/gzip", limiter, nil)
	if err != nil {
		return 0, fmt.Errorf("upload snapshot document: %w", err)
	}

	channelID, accessHash, err := m.StorageChannel()
	if err != nil {
		return msgID, err
	}

	// Pin the newest backup message in the Storage Channel
	_, err = m.api.MessagesUpdatePinnedMessage(ctx, &tg.MessagesUpdatePinnedMessageRequest{
		Peer: &tg.InputPeerChannel{
			ChannelID:  channelID,
			AccessHash: accessHash,
		},
		ID: msgID,
	})
	if err != nil {
		fmt.Printf("Warning: failed to pin backup message (id %d): %v\n", msgID, err)
	}

	// Auto-prune older snapshots to keep the newest 5
	_ = m.PruneSnapshots(ctx, 5)

	return msgID, nil
}

// ListSnapshots retrieves the Snapshot History from the active Storage Channel.
func (m *ClientManager) ListSnapshots(ctx context.Context) ([]SnapshotInfo, error) {
	channelID, accessHash, err := m.StorageChannel()
	if err != nil {
		return nil, err
	}
	return m.ListSnapshotsForPeer(ctx, channelID, accessHash)
}

// ListSnapshotsForPeer retrieves snapshot history for a specific channel peer.
func (m *ClientManager) ListSnapshotsForPeer(ctx context.Context, channelID, accessHash int64) ([]SnapshotInfo, error) {
	peer := &tg.InputPeerChannel{
		ChannelID:  channelID,
		AccessHash: accessHash,
	}

	history, err := m.api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:  peer,
		Limit: 100,
	})
	if err != nil {
		return nil, fmt.Errorf("get channel history: %w", err)
	}

	var snapshots []SnapshotInfo
	switch h := history.(type) {
	case *tg.MessagesChannelMessages:
		for _, message := range h.Messages {
			if msg, ok := message.(*tg.Message); ok {
				if media, ok := msg.Media.(*tg.MessageMediaDocument); ok {
					if doc, ok := media.Document.(*tg.Document); ok {
						for _, attr := range doc.Attributes {
							if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
								if strings.HasPrefix(fn.FileName, "teledrive-backup-") && strings.HasSuffix(fn.FileName, ".db.gz") {
									snapshots = append(snapshots, SnapshotInfo{
										MessageID: msg.ID,
										FileName:  fn.FileName,
										Size:      doc.Size,
										CreatedAt: time.Unix(int64(msg.Date), 0),
										IsPinned:  msg.Pinned,
									})
									break
								}
							}
						}
					}
				}
			}
		}
	}

	// Sort newest first
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.After(snapshots[j].CreatedAt)
	})

	return snapshots, nil
}

// PruneSnapshots removes older snapshots exceeding the retention limit.
func (m *ClientManager) PruneSnapshots(ctx context.Context, keepCount int) error {
	if keepCount <= 0 {
		keepCount = 5
	}
	snapshots, err := m.ListSnapshots(ctx)
	if err != nil {
		return err
	}

	if len(snapshots) <= keepCount {
		return nil
	}

	toDelete := snapshots[keepCount:]
	var msgIDs []int
	for _, s := range toDelete {
		msgIDs = append(msgIDs, s.MessageID)
	}

	channelID, accessHash, err := m.StorageChannel()
	if err != nil {
		return err
	}

	_, err = m.api.ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{
		Channel: &tg.InputChannel{
			ChannelID:  channelID,
			AccessHash: accessHash,
		},
		ID: msgIDs,
	})
	return err
}

// RestoreSnapshotByID restores a specific snapshot by its Telegram message ID.
func (m *ClientManager) RestoreSnapshotByID(ctx context.Context, messageID int, destDBPath string) error {
	channelID, accessHash, err := m.StorageChannel()
	if err != nil {
		return err
	}

	res, err := m.api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
		Channel: &tg.InputChannel{
			ChannelID:  channelID,
			AccessHash: accessHash,
		},
		ID: []tg.InputMessageClass{&tg.InputMessageID{ID: messageID}},
	})
	if err != nil {
		return fmt.Errorf("get message %d: %w", messageID, err)
	}

	var targetDoc *tg.Document
	switch r := res.(type) {
	case *tg.MessagesChannelMessages:
		for _, msgClass := range r.Messages {
			if msg, ok := msgClass.(*tg.Message); ok && msg.ID == messageID {
				if media, ok := msg.Media.(*tg.MessageMediaDocument); ok {
					if doc, ok := media.Document.(*tg.Document); ok {
						targetDoc = doc
						break
					}
				}
			}
		}
	}

	if targetDoc == nil {
		return fmt.Errorf("snapshot with message id %d not found", messageID)
	}

	tempGz := filepath.Join(os.TempDir(), fmt.Sprintf("restored-%d.db.gz", messageID))
	defer os.Remove(tempGz)

	out, err := os.Create(tempGz)
	if err != nil {
		return fmt.Errorf("create temp download: %w", err)
	}
	defer out.Close()

	if err := m.DownloadFull(ctx, targetDoc.ID, targetDoc.AccessHash, out); err != nil {
		return fmt.Errorf("download backup snapshot: %w", err)
	}
	_ = out.Close()

	return db.RestoreFromGzip(tempGz, destDBPath)
}

// DownloadSnapshotByID streams a snapshot's raw gzip bytes into the provided writer.
func (m *ClientManager) DownloadSnapshotByID(ctx context.Context, messageID int, w io.Writer) error {
	channelID, accessHash, err := m.StorageChannel()
	if err != nil {
		return err
	}

	res, err := m.api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
		Channel: &tg.InputChannel{
			ChannelID:  channelID,
			AccessHash: accessHash,
		},
		ID: []tg.InputMessageClass{&tg.InputMessageID{ID: messageID}},
	})
	if err != nil {
		return fmt.Errorf("get message %d: %w", messageID, err)
	}

	var targetDoc *tg.Document
	switch r := res.(type) {
	case *tg.MessagesChannelMessages:
		for _, msgClass := range r.Messages {
			if msg, ok := msgClass.(*tg.Message); ok && msg.ID == messageID {
				if media, ok := msg.Media.(*tg.MessageMediaDocument); ok {
					if doc, ok := media.Document.(*tg.Document); ok {
						targetDoc = doc
						break
					}
				}
			}
		}
	}

	if targetDoc == nil {
		return fmt.Errorf("snapshot with message id %d not found", messageID)
	}

	return m.DownloadFull(ctx, targetDoc.ID, targetDoc.AccessHash, w)
}

// DeleteMessages deletes one or more messages from the Storage Channel.
func (m *ClientManager) DeleteMessages(ctx context.Context, messageIDs ...int) error {
	if len(messageIDs) == 0 {
		return nil
	}
	channelID, accessHash, err := m.StorageChannel()
	if err != nil {
		return err
	}

	_, err = m.api.ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{
		Channel: &tg.InputChannel{
			ChannelID:  channelID,
			AccessHash: accessHash,
		},
		ID: messageIDs,
	})
	return err
}

// DeleteSnapshotByID removes a specific snapshot message from the Storage Channel.
func (m *ClientManager) DeleteSnapshotByID(ctx context.Context, messageID int) error {
	return m.DeleteMessages(ctx, messageID)
}

// RestoreLatestSnapshot searches the Storage Channel for the newest backup snapshot and restores it.
func (m *ClientManager) RestoreLatestSnapshot(ctx context.Context, destDBPath string) error {
	snapshots, err := m.ListSnapshots(ctx)
	if err != nil {
		return err
	}
	if len(snapshots) == 0 {
		return fmt.Errorf("no database backup snapshot found in Storage Channel")
	}

	return m.RestoreSnapshotByID(ctx, snapshots[0].MessageID, destDBPath)
}
