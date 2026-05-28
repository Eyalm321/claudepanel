//go:build windows
package audio

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
)

type WindowsPlayer struct {
	mu       sync.Mutex
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	emit     func(Event)
	ticker   *time.Ticker
	stopChan chan struct{}
	closed   bool
}

func New(emit func(Event)) (Player, error) {
	p := &WindowsPlayer{
		emit:     emit,
		stopChan: make(chan struct{}),
	}

	cmdScript := `
$ErrorActionPreference = "Stop"
[void][Windows.Media.Playback.MediaPlayer, Windows.Media.Playback, ContentType=WindowsRuntime]
[void][Windows.Media.Core.MediaSource, Windows.Media.Core, ContentType=WindowsRuntime]

$player = New-Object Windows.Media.Playback.MediaPlayer

while ($line = [Console]::ReadLine()) {
    try {
        if ($line -like "play *") {
            $url = $line.Substring(5)
            $uri = New-Object System.Uri($url)
            $source = [Windows.Media.Core.MediaSource]::CreateFromUri($uri)
            $player.Source = $source
            $player.Play()
        } elseif ($line -eq "pause") {
            $player.Pause()
        } elseif ($line -eq "stop") {
            $player.Pause()
        } elseif ($line -like "volume *") {
            $volStr = $line.Substring(7)
            $vol = [double]$volStr
            if ($vol -lt 0.0) { $vol = 0.0 }
            if ($vol -gt 1.0) { $vol = 1.0 }
            $player.Volume = $vol
        } elseif ($line -eq "state") {
            Write-Host "STATE:$($player.PlaybackSession.PlaybackState)"
        } elseif ($line -eq "exit") {
            break
        }
    } catch {
        Write-Host "ERROR:$($_.Exception.Message)"
    }
}
`

	cmd := exec.Command("powershell", "-NoProfile", "-Command", cmdScript)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: 0x08000000, // CREATE_NO_WINDOW
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	cmd.Stderr = log.Writer()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start powershell process: %w", err)
	}

	p.cmd = cmd
	p.stdin = stdin

	go p.readStdout(stdout)

	// Start active polling ticker (every 100ms)
	p.ticker = time.NewTicker(100 * time.Millisecond)
	go p.pollLoop()

	return p, nil
}

func (p *WindowsPlayer) readStdout(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "STATE:") {
			stateStr := strings.TrimPrefix(line, "STATE:")
			p.handlePlayState(stateStr)
		} else if strings.HasPrefix(line, "ERROR:") {
			errStr := strings.TrimPrefix(line, "ERROR:")
			p.emit(Event{State: StateError, Err: errStr})
		}
	}
}

func (p *WindowsPlayer) handlePlayState(stateStr string) {
	switch stateStr {
	case "None":
		p.emit(Event{State: StateIdle})
	case "Opening", "Buffering":
		p.emit(Event{State: StateLoading})
	case "Playing":
		p.emit(Event{State: StatePlaying})
	case "Paused":
		p.emit(Event{State: StatePaused})
	}
}

func (p *WindowsPlayer) pollLoop() {
	for {
		select {
		case <-p.ticker.C:
			p.mu.Lock()
			if p.stdin != nil {
				fmt.Fprintln(p.stdin, "state")
			}
			p.mu.Unlock()
		case <-p.stopChan:
			return
		}
	}
}

func (p *WindowsPlayer) Play(url string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.stdin == nil {
		return fmt.Errorf("player not initialized or closed")
	}
	_, err := fmt.Fprintf(p.stdin, "play %s\n", url)
	return err
}

func (p *WindowsPlayer) Pause() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.stdin == nil {
		return fmt.Errorf("player not initialized or closed")
	}
	_, err := fmt.Fprintln(p.stdin, "pause")
	return err
}

func (p *WindowsPlayer) Stop() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.stdin == nil {
		return fmt.Errorf("player not initialized or closed")
	}
	_, err := fmt.Fprintln(p.stdin, "stop")
	return err
}

func (p *WindowsPlayer) SetVolume(v float64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed || p.stdin == nil {
		return fmt.Errorf("player not initialized or closed")
	}
	if v < 0.0 {
		v = 0.0
	} else if v > 1.0 {
		v = 1.0
	}
	_, err := fmt.Fprintf(p.stdin, "volume %f\n", v)
	return err
}

func (p *WindowsPlayer) Close() error {
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return nil
	}
	p.closed = true
	p.mu.Unlock()

	if p.ticker != nil {
		p.ticker.Stop()
	}
	close(p.stopChan)

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.stdin != nil {
		fmt.Fprintln(p.stdin, "exit")
		p.stdin.Close()
		p.stdin = nil
	}
	if p.cmd != nil {
		p.cmd.Process.Kill()
		p.cmd.Wait()
		p.cmd = nil
	}
	return nil
}
