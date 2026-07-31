package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"kaset/internal/library"
	"kaset/internal/player"
	"kaset/internal/playlist"
	"kaset/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "kaset:", err)
		os.Exit(1)
	}
}

func run() error {
	flags := flag.NewFlagSet("kaset", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.Usage = func() {
		fmt.Fprintln(flags.Output(), "Kullanım: kaset [MÜZİK_KLASÖRÜ]")
		fmt.Fprintln(flags.Output(), "Yerel müzikleri tarar ve mpv ile terminalde oynatır.")
	}
	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() > 1 {
		flags.Usage()
		return errors.New("yalnızca bir müzik klasörü verilebilir")
	}

	directory := "."
	if flags.NArg() == 1 {
		directory = flags.Arg(0)
	}
	tracks, err := library.Scan(directory)
	if err != nil {
		return err
	}
	if len(tracks) == 0 {
		return fmt.Errorf("desteklenen ses dosyası bulunamadı: %s", directory)
	}

	playlistStore, err := playlist.DefaultStore()
	if err != nil {
		return err
	}

	mpv, err := player.Start()
	if err != nil {
		return err
	}
	defer mpv.Close()

	program := tea.NewProgram(tui.New(tracks, mpv, playlistStore), tea.WithAltScreen())
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

	_, err = program.Run()
	if errors.Is(err, tea.ErrInterrupted) {
		return nil
	}
	return err
}
