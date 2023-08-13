package ui

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fogleman/ease"
	"github.com/muesli/termenv"
	"github.com/pkg/browser"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

type Spinner interface {
	tea.Model

	ViewFancy() string
	ViewFrame(SpinnerFrames) (string, time.Time, int)
}

type Rave struct {
	// Show extra details useful for debugging a desynced rave.
	ShowDetails bool

	// Address (host:port) on which to listen for auth callbacks.
	AuthCallbackAddr string

	// Configuration for syncing with Spotify.
	SpotifyAuth *spotifyauth.Authenticator

	// File path where tokens will be cached.
	SpotifyTokenPath string

	// The animation to display.
	Frames SpinnerFrames

	// transmits an authenticated Spotify client during the auth callback flow
	spotifyAuthState    string
	spotifyAuthVerifier string
	spotifyAuthCh       chan *spotify.Client

	// the sequence to visualize
	start time.Time
	marks []spotify.Marker

	// syncing along to music
	track *spotify.FullTrack

	// refresh rate
	fps float64

	// current position in the marks sequence
	pos int
}

var _ Spinner = &Rave{}

var colors = []termenv.Color{
	termenv.ANSIRed,
	termenv.ANSIGreen,
	termenv.ANSIYellow,
	termenv.ANSIBlue,
	termenv.ANSIMagenta,
	termenv.ANSICyan,
}

// DefaultBPM is a sane default of 123 beats per minute.
const DefaultBPM = 123

// SpinnerFrames contains animation frames.
type SpinnerFrames struct {
	Frames []string
	Easing ease.Function
}

var MeterFrames = SpinnerFrames{
	[]string{"█", "█", "▇", "▆", "▅", "▄", "▃", "▂", "▁", " "},
	ease.InOutCubic,
}

var FadeFrames = SpinnerFrames{
	[]string{"█", "█", "▓", "▓", "▒", "▒", "░", "░", " ", " "},
	ease.InOutCubic,
}

var DotFrames = SpinnerFrames{
	[]string{"⣾", "⣷", "⣧", "⣏", "⡟", "⡿", "⢿", "⢻", "⣹", "⣼"},
	ease.Linear,
}

var MiniDotFrames = SpinnerFrames{
	[]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	ease.Linear,
}

func NewRave() *Rave {
	r := &Rave{
		Frames: MeterFrames,

		spotifyAuthCh: make(chan *spotify.Client),
	}

	r.reset()

	return r
}

func (rave *Rave) reset() {
	rave.start = time.Now()
	rave.marks = []spotify.Marker{
		{
			Start:    0,
			Duration: 60.0 / DefaultBPM,
		},
	}
	rave.track = nil
	rave.pos = 0
}

type authed struct {
	tok *oauth2.Token
}

func (rave *Rave) Init() tea.Cmd {
	ctx := context.TODO()

	cmds := []tea.Cmd{
		Frame(rave.fps),
		rave.setFPS(DefaultBPM),
		func() tea.Msg {
			client, err := rave.existingAuth(ctx)
			if err == nil {
				return rave.sync(ctx, client)
			}

			return nil
		},
	}

	if rave.AuthCallbackAddr != "" {
		var err error
		rave.spotifyAuthState, err = b64rand(16)
		if err != nil {
			cmds = append(cmds, tea.Printf("failed to generate auth state: %s", err))
			return tea.Batch(cmds...)
		}

		rave.spotifyAuthVerifier, err = b64rand(64)
		if err != nil {
			cmds = append(cmds, tea.Printf("failed to generate verifier: %s", err))
			return tea.Batch(cmds...)
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/auth/spotify", func(w http.ResponseWriter, r *http.Request) {
			tok, err := rave.SpotifyAuth.Token(r.Context(), rave.spotifyAuthState, r,
				oauth2.SetAuthURLParam("code_verifier", rave.spotifyAuthVerifier))
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to get token: %s", err), http.StatusForbidden)
				return
			}

			if st := r.FormValue("state"); st != rave.spotifyAuthState {
				http.Error(w, fmt.Sprintf("bad state: %s != %s", st, rave.spotifyAuthState), http.StatusForbidden)
				return
			}

			err = rave.saveToken(tok)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to save token: %s", err), http.StatusInternalServerError)
				return
			}

			rave.spotifyAuthCh <- spotify.New(rave.SpotifyAuth.Client(r.Context(), tok))
		})

		go http.ListenAndServe(rave.AuthCallbackAddr, mux)
	}

	return tea.Batch(
		cmds...,
	)
}

func (rave *Rave) SpotifyCallbackURL() string {
	return "http://" + rave.AuthCallbackAddr + "/auth/spotify"
}

func (rave *Rave) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case syncedMsg:
		if msg.playing == nil {
			// nothing playing
			return rave, nil
		}

		// update the new timing
		rave.marks = msg.analysis.Beats

		// the Spotify API is strange: Timestamp doesn't do what it's documented to
		// do. the docs say it returns the server timestamp, but that's not at all
		// true. instead it reflects the last time the state was updated.
		//
		// assuming no user interaction, for the first 30 seconds Timestamp will
		// reflect the time the song started, but after 30 seconds it gets bumped
		// once again without the user doing anything. this doesn't happen for
		// every song.
		//
		// since it's going to be wrong half the time no matter what, let's just
		// use the timestamp value directly. :(
		rave.start = time.UnixMilli(msg.playing.Timestamp)

		// rewind to the beginning in case the song changed
		//
		// NB: this doesn't actually reset the sequence shown to the user, it just
		// affects where we start looping through in Progress
		rave.pos = 0

		// save the playing track to enable fancier UI + music status
		rave.track = msg.playing.Item

		// update the FPS appropriately for the track's average tempo
		return rave, rave.setFPS(msg.analysis.Track.Tempo)

	case syncErrMsg:
		return rave, tea.Printf("sync error: %s", msg.err)

	// NB: these might be hit at an upper layer instead; in which case it will
	// *not* propagate.
	case FrameMsg:
		return rave, Frame(rave.fps)
	case SetFPSMsg:
		rave.fps = float64(msg)
		return rave, nil

	// NB: these are captured and forwarded at the outer level.
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, Keys.Rave):
			return rave, rave.Sync()
		case key.Matches(msg, Keys.EndRave):
			return rave, rave.Desync()
		case key.Matches(msg, Keys.ForwardRave):
			rave.start = rave.start.Add(-100 * time.Millisecond)
			rave.pos = 0 // reset and recalculate
			return rave, nil
		case key.Matches(msg, Keys.BackwardRave):
			rave.start = rave.start.Add(100 * time.Millisecond)
			rave.pos = 0 // reset and recalculate
			return rave, nil
		case key.Matches(msg, Keys.Debug):
			rave.ShowDetails = !rave.ShowDetails
			return rave, nil
		}

		return rave, nil

	default:
		return rave, nil
	}
}

func (rave *Rave) setFPS(bpm float64) tea.Cmd {
	bps := bpm / 60.0
	framesPerBeat := len(rave.Frames.Frames)
	fps := bps * float64(framesPerBeat)
	fps *= 2 // decrease chance of missing a frame due to timing
	rave.fps = fps
	return tea.Cmd(func() tea.Msg {
		return SetFPSMsg(fps)
	})
}

func (rave *Rave) View() string {
	frame, _, _ := rave.ViewFrame(rave.Frames)
	return frame
}

func (rave *Rave) ViewFancy() string {
	frame, now, pos := rave.ViewFrame(rave.Frames)
	if rave.ShowDetails {
		frame += " " + rave.viewDetails(now, pos)
	}

	if rave.track != nil && pos != -1 {
		frame = termenv.String(frame).Foreground(colors[pos%len(colors)]).String()
	}

	return frame
}

func (rave *Rave) ViewFrame(frames SpinnerFrames) (string, time.Time, int) {
	framesPerBeat := len(frames.Frames)

	now := time.Now()

	pos, pct := rave.Progress(now)

	frame := int(frames.Easing(pct) * float64(framesPerBeat))

	// some animations go > 100% or <100%, so be defensive and clamp to the
	// frames since that doesn't actually make sense
	if frame < 0 {
		frame = 0
	} else if frame >= framesPerBeat {
		frame = framesPerBeat - 1
	}

	return frames.Frames[frame], now, pos
}

func (model *Rave) viewDetails(now time.Time, pos int) string {
	var out string

	if model.track != nil {
		for i, artist := range model.track.Artists {
			if i > 0 {
				out += ", "
			}
			out += artist.Name
		}

		out += " - " + model.track.Name
	}

	if pos != -1 {
		mark := model.marks[pos]

		elapsed := time.Duration(mark.Start * float64(time.Second))
		dur := time.Duration(mark.Duration * float64(time.Second))
		bpm := time.Minute / dur

		if out != "" {
			out += " "
		}

		out += fmt.Sprintf(
			"%s %dbpm %.1ffps",
			elapsed.Truncate(time.Second),
			bpm,
			model.fps,
		)
	}

	return out
}

func (sched *Rave) Progress(now time.Time) (int, float64) {
	for i := int(sched.pos); i < len(sched.marks); i++ {
		mark := sched.marks[i]

		start := sched.start.Add(time.Duration(mark.Start * float64(time.Second)))

		dur := time.Duration(mark.Duration * float64(time.Second))

		end := start.Add(dur)
		if start.Before(now) && end.After(now) {
			// found the current beat
			sched.pos = i
			return i, float64(now.Sub(start)) / float64(dur)
		}

		if start.After(now) {
			// in between two beats
			return -1, 0
		}
	}

	// reached the end of the beats; replay
	sched.start = now
	sched.pos = 0

	return -1, 0
}

func (rave *Rave) spotifyAuth(ctx context.Context) (*spotify.Client, error) {
	if rave.SpotifyAuth == nil {
		return nil, fmt.Errorf("no auth configured")
	}

	if client, err := rave.existingAuth(ctx); err == nil {
		// user has authenticated previously, no need for auth flow
		return client, nil
	}

	codeChallenge := b64s256([]byte(rave.spotifyAuthVerifier))

	authURL := rave.SpotifyAuth.AuthURL(rave.spotifyAuthState,
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.SetAuthURLParam("code_challenge", codeChallenge),
	)

	if err := browser.OpenURL(authURL); err != nil {
		return nil, fmt.Errorf("open browser: %w", err)
	}

	select {
	case client, ok := <-rave.spotifyAuthCh:
		if !ok {
			return nil, fmt.Errorf("callback error")
		}

		return client, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (m *Rave) existingAuth(ctx context.Context) (*spotify.Client, error) {
	if m.SpotifyAuth == nil || m.SpotifyTokenPath == "" {
		return nil, fmt.Errorf("no auth configured")
	}

	content, err := os.ReadFile(m.SpotifyTokenPath)
	if err != nil {
		return nil, err
	}

	var tok *oauth2.Token
	if err := json.Unmarshal(content, &tok); err != nil {
		return nil, err
	}

	client := spotify.New(m.SpotifyAuth.Client(ctx, tok))

	// check if the token is still valid and/or refresh
	refresh, err := client.Token()
	if err != nil {
		return nil, err
	}

	// save new token if refreshed
	if refresh.AccessToken != tok.AccessToken {
		err = m.saveToken(refresh)
		if err != nil {
			return nil, err
		}
	}

	return client, nil
}

func (m *Rave) Sync() tea.Cmd {
	ctx := context.TODO()

	return func() tea.Msg {
		client, err := m.spotifyAuth(ctx)
		if err != nil {
			return tea.Println("failed to authenticate:", err)
		}

		return m.sync(ctx, client)
	}
}

func (m *Rave) Desync() tea.Cmd {
	if m.SpotifyTokenPath != "" {
		_ = os.Remove(m.SpotifyTokenPath)
	}

	m.reset()

	return nil
}

type syncedMsg struct {
	playing  *spotify.CurrentlyPlaying
	analysis *spotify.AudioAnalysis
}

type syncErrMsg struct {
	err error
}

func (m *Rave) sync(ctx context.Context, client *spotify.Client) tea.Msg {
	playing, err := client.PlayerCurrentlyPlaying(ctx)
	if err != nil {
		return syncErrMsg{fmt.Errorf("get currently playing: %w", err)}
	}

	if playing.Item == nil {
		return syncedMsg{} // no song playing
	}

	analysis, err := client.GetAudioAnalysis(ctx, playing.Item.ID)
	if err != nil {
		return syncErrMsg{fmt.Errorf("failed to get audio analysis: %w", err)}
	}

	return syncedMsg{
		playing:  playing,
		analysis: analysis,
	}
}

func (m *Rave) saveToken(tok *oauth2.Token) error {
	if m.SpotifyTokenPath == "" {
		return nil
	}

	payload, err := json.Marshal(tok)
	if err != nil {
		return err
	}

	_ = os.WriteFile(m.SpotifyTokenPath, payload, 0600)
	if err != nil {
		return err
	}

	return nil
}

func b64s256(val []byte) string {
	h := sha256.New()
	h.Write(val)
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func b64rand(bytes int) (string, error) {
	data := make([]byte, bytes)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(data), nil
}

type FrameMsg time.Time

type SetFPSMsg float64

func Frame(fps float64) tea.Cmd {
	return tea.Tick(time.Duration(float64(time.Second)/fps), func(t time.Time) tea.Msg {
		return FrameMsg(t)
	})
}
