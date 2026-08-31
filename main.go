package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/lipgloss"
	"github.com/dh-kam/kakaotalk-dobby/internal/bootstrap"
)

var errorPrefix = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("9")).Render("ERROR:")

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := bootstrap.NewRootCommand(ctx).ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, errorPrefix, err)
		os.Exit(1)
	}
}
