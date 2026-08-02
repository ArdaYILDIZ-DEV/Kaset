package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"kaset/internal/config"
	"kaset/internal/library"
	"kaset/internal/player"
	"kaset/internal/playlist"
	"kaset/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

type cliOptions struct {
	directory string
	explicit  bool
	help      bool
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "kaset:", err)
		os.Exit(1)
	}
}

func run() error {
	options, err := parseArguments(os.Args[1:], os.Stderr)
	if err != nil || options.help {
		return err
	}

	settingsStore, err := config.DefaultStore()
	if err != nil {
		return err
	}
	settings, settingsErr := settingsStore.Load()
	initialNotices := make([]string, 0, 3)
	if settingsErr != nil {
		var recovery *config.RecoveryError
		if !errors.As(settingsErr, &recovery) {
			return settingsErr
		}
		initialNotices = append(initialNotices, settingsErr.Error())
	}

	directory := options.directory
	if !options.explicit && settings.Library != "" {
		directory = settings.Library
	}
	tracks, issues, scanErr := library.ScanWithIssues(directory)
	if scanErr != nil && !options.explicit && settings.Library != "" {
		initialNotices = append(initialNotices, "Kayıtlı müzik klasörü açılamadı; geçerli klasör kullanıldı")
		directory = "."
		tracks, issues, scanErr = library.ScanWithIssues(directory)
	}
	if scanErr != nil {
		return scanErr
	}
	if len(tracks) == 0 {
		return fmt.Errorf("desteklenen ses dosyası bulunamadı: %s", directory)
	}
	if len(issues) > 0 {
		initialNotices = append(initialNotices, fmt.Sprintf("Tarama sırasında %d yol okunamadı", len(issues)))
	}
	absoluteDirectory, err := filepath.Abs(directory)
	if err != nil {
		return fmt.Errorf("müzik klasörü çözümlenemedi: %w", err)
	}

	playlistStore, err := playlist.DefaultStore()
	if err != nil {
		return err
	}

	mpv, err := player.Start()
	if err != nil {
		return err
	}
	if err := mpv.SetVolume(settings.Volume); err != nil {
		_ = mpv.Close()
		return fmt.Errorf("kayıtlı ses seviyesi uygulanamadı: %w", err)
	}

	model := tui.NewWithOptions(tracks, mpv, tui.Options{
		PlaylistStore: playlistStore,
		LibraryRoot:   absoluteDirectory,
		InitialNotice: strings.Join(initialNotices, " · "),
		InitialVolume: &settings.Volume,
		ShowFolders:   settings.ShowFolders,
	})
	program := tea.NewProgram(model, tea.WithAltScreen())
	hangup := make(chan os.Signal, 1)
	signal.Notify(hangup, syscall.SIGHUP)
	defer signal.Stop(hangup)
	programDone := make(chan struct{})
	defer close(programDone)
	go func() {
		select {
		case <-hangup:
			program.Quit()
		case <-programDone:
		}
	}()

	finalModel, runErr := program.Run()
	if errors.Is(runErr, tea.ErrInterrupted) {
		runErr = nil
	}
	closeErr := mpv.Close()

	volume := settings.Volume
	showFolders := settings.ShowFolders
	if result, ok := finalModel.(tui.Model); ok {
		volume = result.Volume()
		showFolders = result.ShowFolders()
	}
	settingsErr = settingsStore.Save(config.Settings{
		Library:     absoluteDirectory,
		Volume:      volume,
		ShowFolders: showFolders,
	})
	return errors.Join(runErr, closeErr, settingsErr)
}

func parseArguments(arguments []string, output io.Writer) (cliOptions, error) {
	flags := flag.NewFlagSet("kaset", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Kullanım: kaset [MÜZİK_KLASÖRÜ]")
		fmt.Fprintln(flags.Output(), "Yerel müzikleri tarar ve mpv ile terminalde oynatır.")
	}
	if err := flags.Parse(arguments); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return cliOptions{help: true}, nil
		}
		return cliOptions{}, err
	}
	if flags.NArg() > 1 {
		flags.Usage()
		return cliOptions{}, errors.New("yalnızca bir müzik klasörü verilebilir")
	}
	if flags.NArg() == 1 {
		return cliOptions{directory: flags.Arg(0), explicit: true}, nil
	}
	return cliOptions{directory: "."}, nil
}
