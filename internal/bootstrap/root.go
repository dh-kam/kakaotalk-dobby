package bootstrap

import (
	"context"

	"github.com/dh-kam/kakaotalk-dobby/internal/buildinfo"
	"github.com/dh-kam/kakaotalk-dobby/internal/config"
	"github.com/dh-kam/kakaotalk-dobby/internal/usecase/friends"
	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
	"github.com/spf13/cobra"
)

// InitDefaultFriendGuard initializes and registers the default friend identity guard.
func InitDefaultFriendGuard() {
	cfg := config.Load()
	dir := friends.NewFileFriendDirectory(cfg.FriendsMapPath)
	guard := friends.NewDirectoryFriendGuard(friends.DirectoryFriendGuardConfig{
		Directory:  dir,
		AdminEmail: "inzero@hanmail.net",
	})
	kakao.SetDefaultFriendGuard(guard)
}

func init() {
	InitDefaultFriendGuard()
}

// NewRootCommand creates the root cobra command.
func NewRootCommand(ctx context.Context) *cobra.Command {
	InitDefaultFriendGuard()

	rootCmd := &cobra.Command{
		Use:           "kakaobot",
		Short:         "KakaoTalk CLI bot for sending messages, managing users, and handling chatbot skills",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       buildinfo.Version,
	}

	rootCmd.AddCommand(
		newAuthCommand(ctx),
		newSendCommand(ctx),
		newFriendsCommand(ctx),
		newUserCommand(ctx),
		newStorageCommand(ctx),
		newServeCommand(ctx),
		newSkillCommand(ctx),
		newAgentCommand(ctx),
	)

	return rootCmd
}
