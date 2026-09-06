package bootstrap

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/dh-kam/kakaotalk-dobby/internal/config"
	"github.com/dh-kam/kakaotalk-dobby/internal/usecase/skill"
	"github.com/dh-kam/kakaotalk-dobby/pkg/academy"
	"github.com/dh-kam/kakaotalk-dobby/pkg/agent"
	"github.com/dh-kam/kakaotalk-dobby/pkg/ai"
	"github.com/dh-kam/kakaotalk-dobby/pkg/kakao"
	"github.com/dh-kam/kakaotalk-dobby/pkg/scheduler"
	"github.com/dh-kam/kakaotalk-dobby/pkg/school"
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
		ListenAddr             string        `flag:"listen" usage:"Address to listen on for Kakao OpenBuilder skill requests"`
		ChannelID              string        `flag:"channel-id" usage:"KakaoTalk channel search ID"`
		DataDir                string        `flag:"data-dir" usage:"Directory containing schedule and timetable JSON data files"`
		AdminToken             string        `flag:"admin-token" usage:"Administrative API bearer token for uploading and reloading data"`
		AIProvider             string        `flag:"ai-provider" usage:"AI LLM provider (openai, gemini, claude, ollama, grok, deepseek, grok, mock)"`
		AIAPIKey               string        `flag:"ai-api-key" usage:"AI Provider API key"`
		AIBaseURL              string        `flag:"ai-base-url" usage:"Custom AI base URL (e.g. for Ollama http://localhost:11434/v1)"`
		AIModel                string        `flag:"ai-model" usage:"AI Model name"`
		AISystemPrompt         string        `flag:"ai-system-prompt" usage:"AI System Instruction prompt"`
		CallbackEnabled        bool          `flag:"callback-enabled" usage:"Answer via Kakao Callback API when the payload carries a callbackUrl"`
		CallbackWaitingMessage string        `flag:"callback-waiting-message" usage:"Immediate ack message shown while the callback answer is pending"`
		CallbackTimeout        time.Duration `flag:"callback-timeout" usage:"Budget for generating the callback answer before delivering it"`
	}{}

	binder := flagsbinder.NewViperCobraFlagsBinder().
		StringP("listen", "l", ":8080", "Address to listen on").
		String("channel-id", "0xc0de1ab", "KakaoTalk channel search ID").
		String("data-dir", cfg.DataDir, "Directory containing schedule and timetable JSON data files").
		String("admin-token", cfg.AdminToken, "Administrative API bearer token for uploading and reloading data").
		String("ai-provider", cfg.AIProvider, "AI LLM provider (openai, gemini, claude, ollama, grok, deepseek, grok, mock)").
		String("ai-api-key", cfg.AIAPIKey, "AI Provider API key").
		String("ai-base-url", cfg.AIBaseURL, "Custom AI base URL").
		String("ai-model", cfg.AIModel, "AI Model name").
		String("ai-system-prompt", cfg.AISystemPrompt, "AI System Instruction prompt").
		Bool("callback-enabled", true, "Answer via Kakao Callback API when the payload carries a callbackUrl").
		String("callback-waiting-message", skill.DefaultCallbackWaitingMessage, "Immediate ack message shown while the callback answer is pending").
		Duration("callback-timeout", "", 50*time.Second, "Budget for generating the callback answer before delivering it")

	cmd := &cobra.Command{
		Use:           "serve",
		Short:         "Start Kakao i OpenBuilder chatbot skill webhook server with AI & Autonomous Agent integration",
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
			runtimeCfg := config.Load()

			aiKey := opts.AIAPIKey
			if aiKey == "" {
				aiKey = runtimeCfg.AIAPIKey
			}

			aiProvider, err := ai.NewProvider(ai.ProviderConfig{
				ProviderName: opts.AIProvider,
				APIKey:       aiKey,
				BaseURL:      opts.AIBaseURL,
				Model:        opts.AIModel,
			})
			if err != nil {
				return err
			}

			busSvc := academy.NewService()
			_ = busSvc.LoadFromDir(opts.DataDir)

			schoolSvc := school.NewService()
			_ = schoolSvc.LoadFromDir(opts.DataDir)

			// Initialize Scheduler Store & Dispatcher
			var store scheduler.Store
			jobsPath := filepath.Join(opts.DataDir, "jobs.json")
			fileStore, err := scheduler.NewFileStore(jobsPath)
			if err != nil {
				fileStore, err = scheduler.NewFileStore("data/jobs.json")
			}
			if err != nil {
				store = scheduler.NewMemoryStore()
			} else {
				store = fileStore
			}

			kakaoRegistry, _ := runtimeCfg.BuildKakaoRegistry()
			var kakaoClient kakao.Client
			if kakaoRegistry != nil && kakaoRegistry.Count() > 0 {
				kakaoClient, _ = kakaoRegistry.GetDefault()
			} else if runtimeCfg.ClientID != "" {
				kakaoClient = runtimeCfg.BuildKakaoClient()
			}
			dispatcher := scheduler.NewDispatcher(kakaoClient)
			schedEngine := scheduler.NewEngine(store, dispatcher.HandleJob)

			if err := schedEngine.Start(cmd.Context()); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "⚠️ Failed to start scheduler engine: %v\n", err)
			}
			defer schedEngine.Stop()

			// Initialize Agent if Vertex AI is configured (Bedrock remains available via the `agent` command)
			var botAgent agent.Agent
			if runtimeCfg.VertexAPIKey != "" {
				dynamicProv := agent.NewDynamicLLMProvider(agent.DynamicProviderConfig{
					VertexCfg: agent.VertexConfig{
						Project:  runtimeCfg.VertexProject,
						Location: runtimeCfg.VertexLocation,
						APIKey:   runtimeCfg.VertexAPIKey,
					},
					PrefStore: agent.GetDefaultModelPreferenceStore(),
				})

				registry := agent.NewToolRegistry()
				registry.Register(&agent.CurrentTimeTool{})
				registry.Register(&agent.KoreanHolidayTool{})
				registry.Register(&agent.ServerStatusTool{})
				registry.Register(agent.NewBusScheduleTool(busSvc))
				if schoolSvc != nil {
					registry.Register(agent.NewSchoolTimetableTool(schoolSvc))
				}
				registry.Register(agent.NewScheduleNotificationTool(schedEngine))
				registry.Register(agent.NewListSchedulesTool(schedEngine))
				registry.Register(agent.NewUpdateScheduleTool(schedEngine))
				registry.Register(agent.NewCancelScheduleTool(schedEngine))
				registry.Register(agent.NewDeleteScheduleTool(schedEngine))

				botAgent = agent.NewAgent(agent.AgentConfig{
					Provider: dynamicProv,
					Tools:    registry,
						PromptFunc: func() string {
							domainKnowledge := agent.BuildDomainKnowledge(busSvc, schoolSvc)
							return fmt.Sprintf(`You are a helpful, polite KakaoTalk AI assistant for channel @%s.
You have access to tools for checking current time/date/weekday (get_current_time in KST), checking Korean public holidays/business days (check_korean_holiday), looking up academy bus schedules (get_bus_schedule), looking up elementary school class timetables (get_school_timetable), managing notifications/reminders, and checking server status.
Always answer politely, naturally, and concisely in Korean formatted for mobile screens.

%s`, opts.ChannelID, domainKnowledge)
						},
						MaxIterations: 3,
				})
			}

			return skill.NewSkillServeUseCase().Execute(cmd.Context(), skill.SkillServeRequest{
				ListenAddr:             opts.ListenAddr,
				ChannelID:              opts.ChannelID,
				AIProvider:             aiProvider,
				Agent:                  botAgent,
				BusService:             busSvc,
				SchoolService:          schoolSvc,
				Scheduler:              schedEngine,
				KakaoRegistry:          kakaoRegistry,
				SessionStore:           agent.NewMemorySessionStore(50),
				SystemPrompt:           opts.AISystemPrompt,
				DataDir:                opts.DataDir,
				AdminToken:             opts.AdminToken,
				CallbackEnabled:        opts.CallbackEnabled,
				CallbackWaitingMessage: opts.CallbackWaitingMessage,
				CallbackProcessingTime: opts.CallbackTimeout,
				Out:                    cmd.OutOrStdout(),
			})
		},
	}

	binder.SetTo(cmd.Flags())
	maskFlagDefaults(cmd, "admin-token", "ai-api-key")
	return cmd
}
