package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/mikyk10/phosphor/config"
	"github.com/mikyk10/phosphor/handler"
	"github.com/mikyk10/phosphor/route"
	"github.com/mikyk10/phosphor/store"
	"github.com/mikyk10/phosphor/usecase"
	"github.com/spf13/cobra"
)

func Execute(args []string) {
	rootCmd := &cobra.Command{
		Use:   "phosphor",
		Short: "Phosphor AI Pipeline Service",
	}
	rootCmd.SilenceUsage = true
	rootCmd.SetArgs(args)

	rootCmd.AddCommand(newWebRunCommand())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// shutdownTimeout bounds how long in-flight pipeline requests may finish
// after a shutdown signal. Pipelines call paid LLM APIs, so cutting them off
// mid-flight wastes money and leaves `running` rows behind in the history DB.
const shutdownTimeout = 30 * time.Second

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

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			repo := store.NewRepository(db)
			store.StartPruner(ctx, repo, globalCfg.Database.MaxHistory, store.DefaultPruneInterval)

			runner := usecase.NewPipelineRunner(globalCfg, repo)
			uc := usecase.NewPipelineUsecase(svcCfg, runner, repo)
			ph := handler.NewPipelineHandler(uc)

			e := echo.New()
			route.Configure(e, ph, globalCfg.Auth.Token)

			addr := fmt.Sprintf(":%d", globalCfg.Port)
			slog.Info("server starting", "port", globalCfg.Port, "dsn", globalCfg.Database.DSN,
				"pipelines", len(svcCfg.Pipelines))

			s := http.Server{
				Addr:         addr,
				Handler:      e,
				ReadTimeout:  5 * time.Minute,
				WriteTimeout: 5 * time.Minute,
			}

			serveErr := make(chan error, 1)
			go func() {
				if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					serveErr <- err
					return
				}
				serveErr <- nil
			}()

			select {
			case err := <-serveErr:
				return err
			case <-ctx.Done():
			}

			// Stop listening for signals so a second Ctrl-C aborts immediately
			// instead of being swallowed by the shutdown wait.
			stop()
			slog.Info("server shutting down", "grace_period", shutdownTimeout)

			shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
			defer cancel()

			if err := s.Shutdown(shutdownCtx); err != nil {
				slog.Error("server shutdown failed, closing connections", "err", err)
				return err
			}

			// ListenAndServe has returned by now; surface anything unexpected.
			if err := <-serveErr; err != nil {
				return err
			}

			slog.Info("server stopped")
			return nil
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
