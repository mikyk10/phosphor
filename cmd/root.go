package cmd

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/mikyk10/wisp-ai/config"
	"github.com/mikyk10/wisp-ai/handler"
	"github.com/mikyk10/wisp-ai/route"
	"github.com/mikyk10/wisp-ai/store"
	"github.com/mikyk10/wisp-ai/usecase"
	"github.com/spf13/cobra"
)

func Execute(args []string) {
	rootCmd := &cobra.Command{
		Use:   "wisp-ai",
		Short: "WiSP AI Pipeline Service",
	}
	rootCmd.SilenceUsage = true
	rootCmd.SetArgs(args)

	rootCmd.AddCommand(newWebRunCommand())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func newWebRunCommand() *cobra.Command {
	configDir := "./config"

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start the HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			globalCfg, svcCfg, err := config.Load(configDir)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: globalCfg.LogLevel,
			})))

			db, err := store.NewSQLiteConnection(globalCfg.Database.DSN, false)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			if err := db.AutoMigrate(store.AllModels()...); err != nil {
				return fmt.Errorf("auto-migrate: %w", err)
			}

			repo := store.NewRepository(db)
			runner := usecase.NewPipelineRunner(globalCfg, repo)
			uc := usecase.NewPipelineUsecase(svcCfg, runner, repo)
			ph := handler.NewPipelineHandler(uc)

			e := echo.New()
			route.Configure(e, ph)

			addr := fmt.Sprintf(":%d", globalCfg.Port)
			slog.Info("server starting", "port", globalCfg.Port, "dsn", globalCfg.Database.DSN,
				"pipelines", len(svcCfg.Pipelines))

			s := http.Server{
				Addr:         addr,
				Handler:      e,
				ReadTimeout:  5 * time.Minute,
				WriteTimeout: 5 * time.Minute,
			}
			return s.ListenAndServe()
		},
	}

	cmd.Flags().StringVar(&configDir, "config", "./config", "Config directory path")

	webCmd := &cobra.Command{
		Use:   "web",
		Short: "Web server commands",
	}
	webCmd.AddCommand(cmd)

	return webCmd
}
