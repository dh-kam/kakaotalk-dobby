package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"github.com/dh-kam/kakao-bot/internal/config"
	"github.com/dh-kam/kakao-bot/internal/usecase/agent"
	"github.com/dh-kam/refutils/flagsbinder"
	"github.com/spf13/cobra"
)

func newAgentCommand(ctx context.Context) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "agent",
		Short:         "Run autonomous AI Agent with Google Vertex AI or AWS Bedrock models",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(
		newAgentRunCommand(ctx),
	)

	return cmd
}

func newAgentRunCommand(ctx context.Context) *cobra.Command {
	cfg := config.Load()

	opts := struct {
		Provider     string `flag:"provider" usage:"Agent LLM provider (vertex, bedrock, mock)"`
		Model        string `flag:"model" usage:"Model ID (defaults: gemini-3.7-flash for vertex, amazon.nova-pro-v1:0 for bedrock)"`
		GCPProject   string `flag:"gcp-project" usage:"Google Cloud Project ID for Vertex AI"`
		GCPLocation  string `flag:"gcp-location" usage:"Google Cloud Location (e.g. us-central1, asia-northeast3)"`
		GCPToken     string `flag:"gcp-token" usage:"Google Cloud OAuth2 Access Token"`
		APIKey       string `flag:"api-key" usage:"API Key (e.g. VERTEX_API_KEY)"`
		AWSRegion    string `flag:"aws-region" usage:"AWS Region for Bedrock (e.g. us-east-1, ap-northeast-2)"`
		AWSBearer    string `flag:"aws-bearer" usage:"AWS Bedrock Bearer Token / API Key"`
		AWSAccessKey string `flag:"aws-access-key" usage:"AWS Access Key ID"`
		AWSSecretKey string `flag:"aws-secret-key" usage:"AWS Secret Access Key"`
		Prompt       string `flag:"prompt" usage:"User instruction / query for the agent"`
		ClientID     string `flag:"client-id" usage:"Kakao REST API Key"`
		ClientSecret string `flag:"client-secret" usage:"Kakao Client Secret"`
		RedirectURI  string `flag:"redirect-uri" usage:"Kakao OAuth Redirect URI"`
		TokenPath    string `flag:"token-path" usage:"Path to token JSON file"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("provider", "p", "vertex", "Agent LLM provider (vertex, bedrock, mock)").
		StringP("model", "m", "", "Model ID").
		String("gcp-project", cfg.VertexProject, "Google Cloud Project ID for Vertex AI").
		String("gcp-location", cfg.VertexLocation, "Google Cloud Location for Vertex AI").
		String("gcp-token", "", "Google Cloud OAuth2 Access Token").
		String("api-key", cfg.VertexAPIKey, "Google Vertex API Key (VERTEX_API_KEY)").
		String("aws-region", cfg.BedrockRegion, "AWS Region for Bedrock").
		String("aws-bearer", cfg.BedrockBearerToken, "AWS Bedrock Bearer Token / API Key (AWS_BEARER_TOKEN_BEDROCK)").
		String("aws-access-key", "", "AWS Access Key ID").
		String("aws-secret-key", "", "AWS Secret Access Key").
		String("prompt", "", "User instruction / query for the agent").
		StringP("client-id", "c", cfg.ClientID, "Kakao REST API Key").
		String("client-secret", cfg.ClientSecret, "Kakao Client Secret").
		StringP("redirect-uri", "r", cfg.RedirectURI, "Kakao OAuth Redirect URI").
		StringP("token-path", "t", cfg.TokenPath, "Path to token JSON file")

	cmd := &cobra.Command{
		Use:           "run [prompt]",
		Short:         "Execute an autonomous Agent task with step-by-step reasoning and tool calls",
		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if err := binder.BindCommand(cmd, &opts, args...); err != nil {
				_ = cmd.Usage()
				return err
			}
			if len(args) > 0 && opts.Prompt == "" {
				opts.Prompt = strings.Join(args, " ")
			}
			if opts.Prompt == "" {
				_ = cmd.Usage()
				return fmt.Errorf("prompt is required (provide as argument or --prompt flag)")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			bearerToken := opts.GCPToken
			if opts.Provider == "bedrock" && opts.AWSBearer != "" {
				bearerToken = opts.AWSBearer
			}
			return agent.NewAgentRunUseCase().Execute(cmd.Context(), agent.AgentRunRequest{
				ProviderName: opts.Provider,
				Model:        opts.Model,
				Project:      opts.GCPProject,
				Location:     opts.GCPLocation,
				Region:       opts.AWSRegion,
				BearerToken:  bearerToken,
				APIKey:       opts.APIKey,
				AccessKeyID:  opts.AWSAccessKey,
				SecretKey:    opts.AWSSecretKey,
				Prompt:       opts.Prompt,
				ClientID:     opts.ClientID,
				ClientSecret: opts.ClientSecret,
				RedirectURI:  opts.RedirectURI,
				TokenPath:    opts.TokenPath,
				Out:          cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	return cmd
}
