package game

type StateFunc func(msg Message) StateFunc

// Jukebox service interface
type JukeboxServicer interface {
	Init(_ Message) StateFunc
	StartGame(msg Message) StateFunc
	WaitPanelistsToJoin(msg Message) StateFunc
	WaitForReviews(msg Message) StateFunc
	AddSong(msg Message) StateFunc
	IntroduceSong(msg Message) StateFunc
	RevealReviews(msg Message) StateFunc
	StopGame(_ Message) StateFunc
}

type Message struct {
	Command    string
	Text       string
	PlayerName string
	FromID     int64
	ChatID     int64
}

func (m Message) IsEmpty() bool {
	return m.Command == "" &&
		m.Text == "" &&
		m.PlayerName == "" &&
		m.FromID == 0
}
