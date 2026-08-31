package bootstrap

import (
	"context"

	"github.com/dh-kam/kakaotalk-dobby/internal/buildinfo"
	"github.com/spf13/cobra"
)

// NewRootCommand creates the root cobra command.
func NewRootCommand(ctx context.Context) *cobra.Command {
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
