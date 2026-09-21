package telegram

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

// ConsoleAuthenticator implements auth.UserAuthenticator using stdio prompts.
type ConsoleAuthenticator struct{}

func (ConsoleAuthenticator) Phone(ctx context.Context) (string, error) {
	fmt.Print("Enter your Telegram phone number (e.g. +628123456789): ")
	reader := bufio.NewReader(os.Stdin)
	phone, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(phone), nil
}

func (ConsoleAuthenticator) Password(ctx context.Context) (string, error) {
	fmt.Print("Enter your 2FA Cloud Password (if any): ")
	reader := bufio.NewReader(os.Stdin)
	pwd, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(pwd), nil
}

func (ConsoleAuthenticator) AcceptTermsOfService(ctx context.Context, tos tg.HelpTermsOfService) error {
	return nil
}

func (ConsoleAuthenticator) Code(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	fmt.Print("Enter the Telegram verification code received: ")
	reader := bufio.NewReader(os.Stdin)
	code, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(code), nil
}

func (ConsoleAuthenticator) SignUp(ctx context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign-up not supported: please register your account via official Telegram app first")
}

// AuthenticateInteractive performs terminal-based authentication and initiates channel onboarding.
func (m *ClientManager) AuthenticateInteractive(ctx context.Context, reader *bufio.Reader, destDBPath string, forceReauth bool) error {
	flow := auth.NewFlow(ConsoleAuthenticator{}, auth.SendCodeOptions{})

	status, err := m.client.Auth().Status(ctx)
	if err == nil && status.Authorized {
		if forceReauth {
			fmt.Println("Forcing account re-authentication. Revoking previous session...")
			_ = m.Disconnect(ctx)
		} else {
			accountLabel := "current account"
			if status.User != nil {
				if status.User.Phone != "" {
					accountLabel = "+" + status.User.Phone
				} else if status.User.Username != "" {
					accountLabel = "@" + status.User.Username
				}
			}
			fmt.Printf("✓ Account is already authenticated in database session (%s).\n", accountLabel)
			fmt.Print("Do you want to switch to a different Telegram account? [y/N]: ")
			ans, _ := reader.ReadString('\n')
			if trimmed := strings.ToLower(strings.TrimSpace(ans)); trimmed == "y" || trimmed == "yes" {
				fmt.Println("Disconnecting current account...")
				_ = m.Disconnect(ctx)
			} else {
				return m.OnboardStorageChannelInteractive(ctx, reader, destDBPath)
			}
		}
	}

	if err := m.client.Auth().IfNecessary(ctx, flow); err != nil {
		return fmt.Errorf("interactive auth failed: %w", err)
	}

	fmt.Println("✓ MTProto authentication successful. Session encrypted and stored in SQLite.")
	return m.OnboardStorageChannelInteractive(ctx, reader, destDBPath)
}


// EnsureStorageChannel verifies, discovers, or creates the private Storage Channel "TeleDrive Vault".
func (m *ClientManager) EnsureStorageChannel(ctx context.Context) error {
	storedID, err := m.db.GetSetting("storage_channel_id")
	storedHash, errHash := m.db.GetSetting("storage_channel_hash")

	if err == nil && errHash == nil && storedID != "" && storedHash != "" {
		cID, _ := strconv.ParseInt(storedID, 10, 64)
		cHash, _ := strconv.ParseInt(storedHash, 10, 64)
		m.SetStorageChannel(cID, cHash)
		fmt.Printf("✓ Using existing Storage Channel (ID: %d)\n", cID)
		return nil
	}

	if m.configuredChannelID != 0 {
		hash, err := m.ResolveChannelByID(ctx, m.configuredChannelID)
		if err == nil {
			m.SetStorageChannel(m.configuredChannelID, hash)
			_ = m.db.SetSetting("storage_channel_id", strconv.FormatInt(m.configuredChannelID, 10))
			_ = m.db.SetSetting("storage_channel_hash", strconv.FormatInt(hash, 10))
			fmt.Printf("✓ Bound to configured Storage Channel (ID: %d)\n", m.configuredChannelID)
			return nil
		}
	}

	candidates, err := m.DiscoverStorageChannels(ctx)
	if err == nil && len(candidates) == 1 {
		cand := candidates[0]
		m.SetStorageChannel(cand.ID, cand.AccessHash)
		_ = m.db.SetSetting("storage_channel_id", strconv.FormatInt(cand.ID, 10))
		_ = m.db.SetSetting("storage_channel_hash", strconv.FormatInt(cand.AccessHash, 10))
		fmt.Printf("✓ Auto-discovered existing Storage Channel (ID: %d)\n", cand.ID)
		return nil
	} else if err == nil && len(candidates) > 1 {
		// ponytail: In headless mode with duplicate channels, pick the one with the most snapshots.
		best := candidates[0]
		for _, c := range candidates[1:] {
			if c.SnapshotCount > best.SnapshotCount {
				best = c
			}
		}
		m.SetStorageChannel(best.ID, best.AccessHash)
		_ = m.db.SetSetting("storage_channel_id", strconv.FormatInt(best.ID, 10))
		_ = m.db.SetSetting("storage_channel_hash", strconv.FormatInt(best.AccessHash, 10))
		fmt.Printf("✓ Auto-discovered Storage Channel (ID: %d, %d snapshots)\n", best.ID, best.SnapshotCount)
		return nil
	}

	_, _, err = m.CreateStorageChannel(ctx)
	return err
}
