package game

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"weezel/jukeboxjury/internal/integration/telegram"
	"weezel/jukeboxjury/internal/logger"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const JukeboxJuryPrefix = "levyraati"

const (
	CommandStart    = "aloita"
	CommandContinue = "jatka"
	CommandJoin     = "liity"
	CommandPresent  = "(esitä|esitys)"
	CommandReview   = "(arvio|arvioi|arvostele)"
)

var (
	ErrInvalidPrefix = errors.New("invalid prefix")
	ErrInvaliSyntax  = errors.New("invalid syntax")
)

var AllCommands = []string{
	CommandContinue,
	CommandJoin,
	CommandPresent,
	CommandReview,
}

type PlayOption func(*Play)

func WithOutputDirectory(fpath string) PlayOption {
	return func(p *Play) {
		p.resultsDirectory = &fpath
	}
}

func WithResultsURL(resultsURL string) PlayOption {
	return func(p *Play) {
		p.resultsURL = &resultsURL
	}
}

// Play implements JukeboxServicer interface
type Play struct {
	host              *Panelist
	StartedAt         time.Time
	bot               telegram.Boter
	resultsDirectory  *string
	resultsURL        *string
	Panelists         []*Panelist
	gameStarterUID    int64
	chatID            int64
	allSongsSubmitted bool
	gameActive        bool
}

func New(bot telegram.Boter, chatID int64, opts ...PlayOption) *Play {
	resultsURL := "http://127.0.0.1"
	homeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Logger.Error().Err(err).Msg("Failed to get home directory, fallback to /tmp")
		homeDir = "/tmp"
	}
	g := &Play{
		bot:              bot,
		chatID:           chatID,
		resultsDirectory: &homeDir,
		resultsURL:       &resultsURL,
		Panelists:        []*Panelist{},
	}

	// Override defaults with given options
	for _, opt := range opts {
		opt(g)
	}

	return g
}

// States
func (p *Play) Init(_ Message) StateFunc {
	return p.StartGame
}

func (p *Play) StartGame(msg Message) StateFunc {
	logger.Logger.Debug().Msg("State: Game is starting")

	// Game is about to be started, show intro
	if p.StartedAt.IsZero() {
		p.gameActive = true
		p.StartedAt = time.Now().Local()
		p.gameStarterUID = msg.FromID
		p.sendMessageToChannel(
			fmt.Sprintf("User %s started a new game, join by using command: %s %s",
				msg.PlayerName, JukeboxJuryPrefix, CommandJoin,
			),
		)
		logger.Logger.Info().
			Str("game_starter_name", msg.PlayerName).
			Int64("game_starter_id", msg.FromID).
			Time("game_starter_at", p.StartedAt).
			Msg("Game started")
		p.addPanelist(msg)
	}

	return p.WaitPanelistsToJoin
}

func (p *Play) WaitPanelistsToJoin(msg Message) StateFunc {
	logger.Logger.Debug().Msg("State: Waiting panelists to join")

	switch msg.Command {
	case CommandJoin:
		if ok := p.addPanelist(msg); !ok {
			p.sendMessageToPanelist(msg.ChatID, "You are already in the game")
		} else {
			p.sendMessageToChannel(fmt.Sprintf("User %s joined the game", msg.PlayerName))
		}
	case CommandContinue:
		// TODO Add higher bit for the user who started the game?
		logger.Logger.Info().Msg("Panelists are ready, continuing")
		p.sendMessageToChannel(fmt.Sprintf("User %s wants to proceed, continuing...", msg.PlayerName))
		return p.AddSong
	}

	return p.WaitPanelistsToJoin
}

var songCommandMatcher = regexp.MustCompile(CommandPresent)

func (p *Play) AddSong(msg Message) StateFunc {
	logger.Logger.Debug().Msg("State: Add song")

	if !songCommandMatcher.MatchString(msg.Command) {
		logger.Logger.Warn().Interface("msg", msg).Msg("Not a command")
		p.sendMessageToPanelist(msg.ChatID, "Aww cute, but it's a wrong command.")
		return p.AddSong
	}

	if err := p.addSong(msg); err != nil {
		var songErr SongError
		if errors.As(err, &songErr) {
			p.sendMessageToPanelist(msg.ChatID, songErr.ErrForUser)
		} else {
			logger.Logger.Error().Err(err).Interface("msg", msg).Msg("Couldn't add song")
			p.sendMessageToPanelist(msg.ChatID, "Me confused. Que pasa¿")
		}
		return p.AddSong
	}
	logger.Logger.Info().
		Interface("msg", msg).
		Msgf("Panelist %s with ID %d added a song", msg.PlayerName, msg.FromID)
	p.sendMessageToChannel(fmt.Sprintf("Panelist %s added a song", msg.PlayerName))

	if !p.allSongsSubmitted {
		return p.AddSong
	}

	logger.Logger.Info().Msg("All songs submitted, continuing")
	p.sendMessageToChannel("All songs submitted, continuing...")

	p.shuffleHost()

	return p.IntroduceSong(msg)
}

// IntroduceSong reveals the next song for the audience.
func (p *Play) IntroduceSong(_ Message) StateFunc {
	logger.Logger.Debug().Msg("State: Introduce the song")

	for _, panelist := range p.Panelists {
		if panelist.SongPresented {
			continue
		}

		p.host = panelist
		panelist.SongPresented = true
		panelist.ReviewGiven = true // Cannot review yourself

		logger.Logger.Info().
			Str("hosts_name", p.host.Name).
			Str("hosts_song", p.host.Song.String()).
			Msg("Current presenter")

		p.sendMessageToChannel(
			fmt.Sprintf("The next song comes from the panelist %s and the song's details: %s",
				panelist.Name, panelist.Song.String(),
			),
		)
		return p.WaitForReviews
	}

	return p.WaitForReviews
}

var reviewCommandMatcher = regexp.MustCompile(CommandReview)

func (p *Play) WaitForReviews(msg Message) StateFunc {
	logger.Logger.Debug().Msg("State: Review and rate the song")

	if !reviewCommandMatcher.MatchString(msg.Command) {
		logger.Logger.Warn().Interface("msg", msg).Msg("Not a command")
		p.sendMessageToChannel("Aww cute, but it's a wrong command.")
		return p.WaitForReviews
	}

	if p.host.uid == msg.FromID {
		logger.Logger.Warn().Msgf("Panelist %s with ID %d tried to rate own song",
			msg.PlayerName,
			msg.FromID,
		)
		p.sendMessageToPanelist(msg.ChatID, "You naughty. It's not possible to review own songs")
		return p.WaitForReviews
	}

	var reviewer *Panelist
	for _, rev := range p.Panelists {
		if msg.FromID == rev.uid {
			reviewer = rev
			break
		}
	}
	if reviewer == nil {
		logger.Logger.Error().Msgf("Couldn't find matching ID for user %s with ID %d",
			msg.PlayerName, msg.FromID,
		)
		return p.WaitForReviews
	}

	if err := p.host.AddReview(reviewer, msg.Text); err != nil {
		reviewErr := ReviewError{}
		if errors.As(err, &reviewErr) {
			logger.Logger.Error().Err(reviewErr.Err).Msg("Couldn't parse review")
			p.sendMessageToPanelist(msg.ChatID, reviewErr.ErrForUser)
		} else {
			logger.Logger.Error().Err(err).Interface("msg", msg).Msg("Couldn't add review")
			p.sendMessageToPanelist(msg.ChatID, "Me confused two times. Que pasa¿")
		}
		return p.WaitForReviews
	}
	logger.Logger.Info().
		Str("host_name", p.host.Name).
		Interface("received_reviews", p.host.ReceivedReviews).
		Msgf("Panelist %s reviewed the song %s", msg.PlayerName, p.host.Song.URL)
	p.sendMessageToChannel(fmt.Sprintf("Panelist %s reviewed the song", msg.PlayerName))

	if !p.isCurrentRoundReviewsDone() {
		return p.WaitForReviews
	}

	logger.Logger.Info().Msgf("Everybody has reviewed the song %s", p.host.Song.URL)
	p.sendMessageToChannel("Everybody has reviewed the song, continuing...")

	return p.RevealReviews(msg) // Immediate transition
}

func (p *Play) RevealReviews(_ Message) StateFunc {
	logger.Logger.Debug().Msg("State: Reveal the song reviews")

	for _, r := range p.host.ReceivedReviews {
		if os.Getenv("TEST_ENV") != "" {
			// Avoid throttling
			//nolint:gosec // False positive nagging, we do _use_ rand/v2
			time.Sleep(time.Millisecond * time.Duration(rand.Int64N(800)))
		}

		review := fmt.Sprintf("%s wrote: %s. The song rating was: %d/10", r.From, r.Review, r.Rating)
		p.sendMessageToChannel(review)
	}

	p.countSongAverageScore()
	finalScore := fmt.Sprintf("Eventually the song %s ended up catching %0.2f points",
		p.host.Song.URL,
		p.host.Song.AverageScore,
	)
	p.sendMessageToChannel(finalScore)

	lastPanelist := p.Panelists[len(p.Panelists)-1]
	if lastPanelist.uid != p.host.uid {
		// For the next round, wipe given reviews flags
		for _, panelist := range p.Panelists {
			panelist.ReviewGiven = false
		}
		return p.IntroduceSong(Message{})
	}

	return p.StopGame(Message{})
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return err == nil
}

func (p *Play) StopGame(_ Message) StateFunc {
	p.sendMessageToChannel("State: Ending the game")

	logger.Logger.Info().
		Interface("output", p.Panelists).
		Dur("duration", time.Since(p.StartedAt)).
		Msg("Game results")

	if p.resultsDirectory != nil { //nolint:nestif // Not that complex?
		fout, fname, err := p.createResultsFile()
		if err != nil {
			logger.Logger.Error().Err(err).Msg("Failed to create results file")
			p.sendMessageToChannel("Failed to create results file")
		} else {
			if err = renderResults(*p, fout); err != nil {
				logger.Logger.Error().Err(err).Msg("Rendering the results failed")
				p.sendMessageToChannel("Failed to render the results")
			} else {
				p.sendMessageToChannel(
					fmt.Sprintf("Results are available in %s", filepath.Join(*p.resultsURL, fname)),
				)
			}
		}
		fout.Close()
	} else {
		if err := renderResults(*p, os.Stdout); err != nil {
			logger.Logger.Error().Err(err).Msg("Rendering the results failed")
			p.sendMessageToChannel("Failed to render the results")
		}
	}

	winner := p.Panelists[0]
	for i := 1; i < len(p.Panelists)-1; i++ {
		panelist := p.Panelists[i]
		if panelist.Song.AverageScore > winner.Song.AverageScore {
			winner = panelist
		}
	}
	p.sendMessageToChannel(
		fmt.Sprintf("Game has ended. The winner song came from %s and was %s with %.2f average score",
			winner.Name,
			winner.Song.URL,
			winner.Song.AverageScore,
		),
	)

	p.gameActive = false

	return nil
}

func (p *Play) createResultsFile() (*os.File, string, error) {
	fname := fmt.Sprintf("jukebox_jury_results_%s.html", time.Now().Local().Format("2006-01-02T15:04:05"))
	fpath := filepath.Join(*p.resultsDirectory, fname)
	if fileExists(fpath) {
		return nil, "", fmt.Errorf("file %q already exists", fpath)
	}

	fout, err := os.Create(fpath)
	if err != nil {
		return nil, "", fmt.Errorf("file %q creation: %w", fpath, err)
	}

	return fout, fname, nil
}

func (p *Play) countSongAverageScore() {
	if len(p.host.ReceivedReviews) == 1 {
		p.host.Song.AverageScore = float64(p.host.ReceivedReviews[0].Rating)
	} else {
		sum := 0
		for _, r := range p.host.ReceivedReviews {
			sum += r.Rating
		}
		p.host.Song.AverageScore = float64(sum) / float64(len(p.host.ReceivedReviews))
	}

	logger.Logger.Info().
		Str("song_presenter", p.host.Name).
		Float64("song_average_score", p.host.Song.AverageScore).
		Msgf("Counted scores for the song %s", p.host.Song.URL)
}

func (p *Play) addPanelist(msg Message) bool {
	for _, p := range p.Panelists {
		if p.uid == msg.FromID {
			logger.Logger.Info().Msgf("Panelist %s with ID %d has already joined the game",
				msg.PlayerName,
				msg.FromID,
			)
			return false
		}
	}
	p.Panelists = append(p.Panelists, NewPanelist(msg.PlayerName, msg.FromID))
	logger.Logger.Info().Msgf("Panelist %s with ID %d joined the game", msg.PlayerName, msg.FromID)

	return true
}

type SongError struct {
	Err        string
	ErrForUser string
}

func (s SongError) Error() string {
	return fmt.Sprintf("add song: %s", s.Err)
}

func (p *Play) addSong(msg Message) error {
	panelist := &Panelist{}
	for _, pan := range p.Panelists {
		if pan.uid != msg.FromID {
			continue
		}

		if !pan.SongSubmitted {
			panelist = pan
			break
		}

		return SongError{
			ErrForUser: "Song already added",
			Err: fmt.Sprintf("panelist %s with ID %d has already added the song",
				msg.PlayerName,
				msg.FromID,
			),
		}
	}
	if panelist == nil {
		return SongError{
			ErrForUser: "Was, bist du echt?",
			Err: fmt.Sprintf("panelist %s with ID %d tried to add song, although not in the game",
				msg.PlayerName,
				msg.FromID,
			),
		}
	}

	if err := panelist.AddSong(msg); err != nil {
		logger.Logger.Warn().
			Interface("msg", msg).
			Msgf("Panelist %s with ID %d presented malformed song", msg.PlayerName, msg.FromID)
		return SongError{
			ErrForUser: "Song given in the malformed form",
			Err: fmt.Sprintf("panelist %s with ID %d presented malformed song: %q",
				msg.PlayerName,
				msg.FromID,
				msg.Text,
			),
		}
	}

	if p.isAllSongsSubmitted() {
		p.allSongsSubmitted = true
	}

	return nil
}

func (p *Play) shuffleHost() {
	// In test mode shuffle is skipped to make order comparable
	if os.Getenv("TEST_MODE") != "" {
		return
	}

	logger.Logger.Debug().Msg("State: Shuffle the host")

	rand.Shuffle(len(p.Panelists), func(i, j int) {
		p.Panelists[i], p.Panelists[j] = p.Panelists[j], p.Panelists[i]
	})
}

// sendMessageToChannel sends text parameter to Telegram channel and logs failed sends.
func (p *Play) sendMessageToChannel(text string) {
	msg := tgbotapi.NewMessage(p.chatID, text)
	_, err := p.bot.Send(msg)
	if err != nil {
		logger.Logger.Error().Err(err).Str("payload", text).Msg("Error sending channel message")
		return
	}
}

// sendMessageToPanelist sends text parameter to panelist and logs failed sends.
func (p *Play) sendMessageToPanelist(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := p.bot.Send(msg)
	if err != nil {
		logger.Logger.Error().Err(err).Str("payload", text).Msg("Error sending user message")
		return
	}
}

func (p *Play) isAllSongsSubmitted() bool {
	for _, panelist := range p.Panelists {
		if !panelist.SongSubmitted {
			return false
		}
	}

	return true
}

func (p *Play) isCurrentRoundReviewsDone() bool {
	expectedReviewsCount := len(p.Panelists) - 1
	if len(p.host.ReceivedReviews) != expectedReviewsCount {
		logger.Logger.Debug().
			Str("host_name", p.host.Name).
			Interface("received_reviews", p.host.ReceivedReviews).
			Msgf("Expected reviews %d, so far received %d",
				expectedReviewsCount,
				len(p.host.ReceivedReviews),
			)
		return false
	}

	return true
}

func ParseToMessage(u tgbotapi.Update) (Message, error) {
	msg := Message{}

	splt := strings.SplitN(u.Message.Text, " ", 3)
	switch len(splt) {
	case 2:
		msg.Command = splt[1]
		msg.Text = strings.Join(splt[2:], " ")
	case 3:
		msg.Command = splt[1]
		msg.Text = strings.Join(splt[2:], " ")
	default:
		return Message{}, ErrInvaliSyntax
	}

	if splt[0] != JukeboxJuryPrefix {
		return Message{}, ErrInvaliSyntax
	}

	if u.Message.From.UserName == "" {
		msg.PlayerName = u.Message.From.FirstName
	} else {
		msg.PlayerName = u.Message.From.UserName
	}

	msg.FromID = u.Message.From.ID
	msg.ChatID = u.Message.Chat.ID

	return msg, nil
}

// func HostNewGame(g *GamePlay, bot TelegramBoter) {
// 	state := g.Init
// 	u := tgbotapi.NewUpdate(0)
// 	u.Timeout = 30
// 	for update := range bot.GetUpdatesChan(u) {
// 		if update.Message == nil {
// 			continue
// 		}

// 		if strings.Fields(update.Message.Text)[0] != jukeboxJuryPrefix {
// 			continue
// 		}

// 		tmp := strings.SplitN(update.Message.Text, " ", 2)
// 		if len(tmp) < 2 {
// 			continue
// 		}
// 		cmd := tmp[1]
// 		// msg := tmp[2:]

// 		switch cmd {
// 		case "aloita":
// 			g = New(bot, update.Message.Chat.ID, update.Message.From.ID)
// 			state = g.StartGame
// 			logger.Logger.Info().Msg("Starting a new game")

// 		case "lopeta":
// 			g.StopGame(update)
// 			logger.Logger.Info().Msg("Game stopped")
// 			g = nil
// 		}

// 		state = state(update)
// 	}
// }
