package bootstrap

import (
	"context"

	"github.com/dh-kam/kakao-bot/internal/config"
	"github.com/dh-kam/kakao-bot/internal/usecase/skill"
	"github.com/dh-kam/kakao-bot/pkg/ai"
	"github.com/dh-kam/refutils/flagsbinder"
	"github.com/spf13/cobra"
)

func newSkillCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "skill",
		Short:         "Manage and run Kakao i OpenBuilder chatbot skill webhook server",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		newSkillServeCommand(ctx),
	)

	return cmd
}

func newSkillServeCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		ListenAddr     string `flag:"listen" usage:"Address to listen on for Kakao OpenBuilder skill requests"`
		ChannelID      string `flag:"channel-id" usage:"KakaoTalk channel search ID"`
		AIProvider     string `flag:"ai-provider" usage:"AI LLM provider (openai, gemini, claude, ollama, groq, deepseek, mock)"`
		AIAPIKey       string `flag:"ai-api-key" usage:"AI Provider API key"`
		AIBaseURL      string `flag:"ai-base-url" usage:"Custom AI base URL (e.g. for Ollama http://localhost:11434/v1)"`
		AIModel        string `flag:"ai-model" usage:"AI Model name"`
		AISystemPrompt string `flag:"ai-system-prompt" usage:"AI System Instruction prompt"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("listen", "l", ":8080", "Address to listen on").
		String("channel-id", "0xc0de1ab", "KakaoTalk channel search ID").
		String("ai-provider", cfg.AIProvider, "AI LLM provider (openai, gemini, claude, ollama, groq, deepseek, mock)").
		String("ai-api-key", cfg.AIAPIKey, "AI Provider API key").
		String("ai-base-url", cfg.AIBaseURL, "Custom AI base URL").
		String("ai-model", cfg.AIModel, "AI Model name").
		String("ai-system-prompt", cfg.AISystemPrompt, "AI System Instruction prompt")

	cmd := &cobra.Command{
		Use:           "serve",
		Short:         "Start Kakao i OpenBuilder chatbot skill webhook server with AI integration",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := binder.BindCommand(cmd, &opts, args...); err != nil {
				_ = cmd.Usage()
				return err
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			aiProvider, err := ai.NewProvider(ai.ProviderConfig{
				ProviderName: opts.AIProvider,
				APIKey:       opts.AIAPIKey,
				BaseURL:      opts.AIBaseURL,
				Model:        opts.AIModel,
			})
			if err != nil {
				return err
			}

			return skill.NewSkillServeUseCase().Execute(cmd.Context(), skill.SkillServeRequest{
				ListenAddr:   opts.ListenAddr,
				ChannelID:    opts.ChannelID,
				AIProvider:   aiProvider,
				SystemPrompt: opts.AISystemPrompt,
				Out:          cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	return cmd
}
