package game

import (
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/go-cmp/cmp"
)

type mockTelegramBot struct {
	mGetUpdatesChan  func(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel
	mSend            func(c tgbotapi.Chattable) (tgbotapi.Message, error)
	receivedMessages []string
}

func (m *mockTelegramBot) Send(c tgbotapi.Chattable) (tgbotapi.Message, error) {
	if msg, ok := c.(tgbotapi.MessageConfig); ok {
		m.receivedMessages = append(m.receivedMessages, msg.Text)
	}

	return m.mSend(c)
}

func (m *mockTelegramBot) GetUpdatesChan(config tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
	return m.mGetUpdatesChan(config)
}

func TestGamePlayWith3Panelists(t *testing.T) {
	t.Helper()

	// Skip shuffling the host to keep tests deterministic
	t.Setenv("TEST_MODE", "true")

	mockBot := mockTelegramBot{
		mGetUpdatesChan: func(_ tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
			return nil
		},
		mSend: func(_ tgbotapi.Chattable) (tgbotapi.Message, error) {
			return tgbotapi.Message{}, nil
		},
		receivedMessages: []string{},
	}

	panelistSantana := &tgbotapi.User{ID: 666, UserName: "Santana"}
	panelistJesus := &tgbotapi.User{ID: 123, UserName: "Jesus"}
	panelistPjotr := &tgbotapi.User{ID: 7, UserName: "Pjotr"}

	introduceCommand := "esitä"
	reviewCommand := "arvioi"
	updates := []tgbotapi.Message{
		// Start game
		{
			Chat: &tgbotapi.Chat{ID: 1},
			From: panelistSantana,
			Text: JukeboxJuryPrefix + " " + CommandStart,
		},
		// Join into the game
		{
			Chat: &tgbotapi.Chat{ID: 3},
			From: panelistPjotr,
			Text: JukeboxJuryPrefix + " " + CommandJoin,
		},
		{
			Chat: &tgbotapi.Chat{ID: 2},
			From: panelistJesus,
			Text: JukeboxJuryPrefix + " " + CommandJoin,
		},
		// Continue to introduce songs
		{
			Chat: &tgbotapi.Chat{ID: 1},
			From: panelistSantana,
			Text: JukeboxJuryPrefix + " " + CommandContinue,
		},
		// Introduce songs
		{
			Chat: &tgbotapi.Chat{ID: 1},
			From: panelistSantana,
			Text: fmt.Sprintf("%s %s My favourite song https://example.com/satan_you_rock",
				JukeboxJuryPrefix, introduceCommand),
		},
		{
			Chat: &tgbotapi.Chat{ID: 3},
			From: panelistPjotr,
			Text: fmt.Sprintf("%s %s I happen to like it https://example.com/pjotr",
				JukeboxJuryPrefix, introduceCommand),
		},
		{
			Chat: &tgbotapi.Chat{ID: 2},
			From: panelistJesus,
			Text: fmt.Sprintf("%s %s Hallelujah 🤘 https://example.com/hesus",
				JukeboxJuryPrefix, introduceCommand),
		},
		// First song reviews, host Santana
		{
			Chat: &tgbotapi.Chat{ID: 2},
			From: panelistJesus,
			Text: fmt.Sprintf("%s %s Great song1 10/10", JukeboxJuryPrefix, reviewCommand),
		},
		{
			Chat: &tgbotapi.Chat{ID: 3},
			From: panelistPjotr,
			Text: fmt.Sprintf("%s %s Nice song such wow1 5/10", JukeboxJuryPrefix, reviewCommand),
		},
		// Second song reviews, host Pjotr
		{
			Chat: &tgbotapi.Chat{ID: 2},
			From: panelistJesus,
			Text: fmt.Sprintf("%s %s Great song2 10/10", JukeboxJuryPrefix, reviewCommand),
		},
		{
			Chat: &tgbotapi.Chat{ID: 1},
			From: panelistSantana,
			Text: fmt.Sprintf("%s %s Terrible song2 1/10", JukeboxJuryPrefix, reviewCommand),
		},
		// The third song review, host Jesus
		{
			Chat: &tgbotapi.Chat{ID: 1},
			From: panelistSantana,
			Text: fmt.Sprintf("%s %s Terrible song3 1/10", JukeboxJuryPrefix, reviewCommand),
		},
		{
			Chat: &tgbotapi.Chat{ID: 3},
			From: panelistPjotr,
			Text: fmt.Sprintf("%s %s Nice song such wow3 5/10", JukeboxJuryPrefix, reviewCommand),
		},
	}

	p := New(&mockBot, 12345678, WithOutputDirectory(nil))
	state := p.StartGame

	for i, update := range updates {
		msg, err := ParseToMessage(tgbotapi.Update{Message: &update})
		if err != nil {
			t.Fatalf("Failed to parse %d %q: %#v", i, update.Text, err)
		}

		fnPtr := reflect.ValueOf(state).Pointer()
		fnName := strings.Split(runtime.FuncForPC(fnPtr).Name(), ".")[2]
		fmt.Println(i, "STATE BEFORE:", fnName)

		state = state(msg)

		fnPtr = reflect.ValueOf(state).Pointer()
		if fnPtr != 0 {
			fnName = strings.Split(runtime.FuncForPC(fnPtr).Name(), ".")[2]
			fmt.Println(i, "STATE AFTER:", fnName)
		} else {
			fmt.Println(i, "STATE AFTER: nil")
		}
	}

	expectedMessages := []string{
		fmt.Sprintf("User Santana started a new game, join by using command: %s %s",
			JukeboxJuryPrefix, CommandJoin),
		"User Pjotr joined the game",
		"User Jesus joined the game",
		"User Santana wants to proceed, continuing...",
		"Add song with the following command and format in private chat with the bot: (esitä|esitys) " +
			"description here https://link-as-last-item",
		"Add review similar way (max score is 5, only integers): (arvio|arvioi|arvostele) description here 5/5",
		"Panelist Santana added a song",
		"Panelist Pjotr added a song",
		"Panelist Jesus added a song",
		"All songs submitted, continuing...",
		// First song introduction & review
		"The next song comes from the panelist Santana and the song's details: Description: " +
			"My favourite song, URL: https://example.com/satan_you_rock",
		"Panelist Jesus reviewed the song",
		"Panelist Pjotr reviewed the song",
		"Everybody has reviewed the song, continuing...",
		"Jesus wrote: Great song1. The song rating was: 10/10",
		"Pjotr wrote: Nice song such wow1. The song rating was: 5/10",
		"Eventually the song https://example.com/satan_you_rock ended up catching 7.50 points",
		// Second song introduction & review
		"The next song comes from the panelist Pjotr and the song's details: Description: " +
			"I happen to like it, URL: https://example.com/pjotr",
		"Panelist Jesus reviewed the song",
		"Panelist Santana reviewed the song",
		"Everybody has reviewed the song, continuing...",
		"Jesus wrote: Great song2. The song rating was: 10/10",
		"Santana wrote: Terrible song2. The song rating was: 1/10",
		"Eventually the song https://example.com/pjotr ended up catching 5.50 points",
		// Third song introduction & review
		"The next song comes from the panelist Jesus and the song's details: Description: Hallelujah 🤘, " +
			"URL: https://example.com/hesus",
		"Panelist Santana reviewed the song",
		"Panelist Pjotr reviewed the song",
		"Everybody has reviewed the song, continuing...",
		"Santana wrote: Terrible song3. The song rating was: 1/10",
		"Pjotr wrote: Nice song such wow3. The song rating was: 5/10",
		"Eventually the song https://example.com/hesus ended up catching 3.00 points",
		"State: Ending the game",
		"Game has ended. The winner song came from Santana and was https://example.com/satan_you_rock " +
			"with 7.50 average score",
	}

	if len(expectedMessages) != len(mockBot.receivedMessages) {
		t.Fatalf("Length of the expected messages %d differs from received %d, diff:\n%s\n",
			len(expectedMessages),
			len(mockBot.receivedMessages),
			cmp.Diff(expectedMessages, mockBot.receivedMessages),
		)
	}

	for i := range expectedMessages {
		if mockBot.receivedMessages[i] != expectedMessages[i] {
			t.Fatalf("Message index=%d, expected=%q, got=%q",
				i, mockBot.receivedMessages[i], expectedMessages[i],
			)
		}
	}
}

func TestGamePlayWith2Panelists(t *testing.T) {
	t.Helper()

	// Skip shuffling the host to keep tests deterministic
	t.Setenv("TEST_MODE", "true")

	mockBot := mockTelegramBot{
		mGetUpdatesChan: func(_ tgbotapi.UpdateConfig) tgbotapi.UpdatesChannel {
			return nil
		},
		mSend: func(_ tgbotapi.Chattable) (tgbotapi.Message, error) {
			return tgbotapi.Message{}, nil
		},
		receivedMessages: []string{},
	}

	panelistSantana := &tgbotapi.User{ID: 666, UserName: "Santana"}
	panelistJesus := &tgbotapi.User{ID: 123, UserName: "Jesus"}

	introduceCommand := "esitä"
	reviewCommand := "arvioi"
	updates := []tgbotapi.Message{
		// Start game
		{
			Chat: &tgbotapi.Chat{ID: 1},
			From: panelistSantana,
			Text: JukeboxJuryPrefix + " " + CommandStart,
		},
		// Join into the game
		{
			Chat: &tgbotapi.Chat{ID: 2},
			From: panelistJesus,
			Text: JukeboxJuryPrefix + " " + CommandJoin,
		},
		// Continue to introduce songs
		{
			Chat: &tgbotapi.Chat{ID: 1},
			From: panelistSantana,
			Text: JukeboxJuryPrefix + " " + CommandContinue,
		},
		// Introduce songs
		{
			Chat: &tgbotapi.Chat{ID: 1},
			From: panelistSantana,
			Text: fmt.Sprintf("%s %s My favourite song https://example.com/satan_you_rock",
				JukeboxJuryPrefix, introduceCommand),
		},
		{
			Chat: &tgbotapi.Chat{ID: 2},
			From: panelistJesus,
			Text: fmt.Sprintf("%s %s Hallelujah 🤘 https://example.com/hesus",
				JukeboxJuryPrefix, introduceCommand),
		},
		// First song review, host Santana
		{
			Chat: &tgbotapi.Chat{ID: 2},
			From: panelistJesus,
			Text: fmt.Sprintf("%s %s Great song1 10/10", JukeboxJuryPrefix, reviewCommand),
		},
		// The second song review, host Jesus
		{
			Chat: &tgbotapi.Chat{ID: 1},
			From: panelistSantana,
			Text: fmt.Sprintf("%s %s Terrible song1 1/10", JukeboxJuryPrefix, reviewCommand),
		},
	}

	p := New(&mockBot, 123456789, WithOutputDirectory(nil))
	state := p.StartGame

	for i, update := range updates {
		msg, err := ParseToMessage(tgbotapi.Update{Message: &update})
		if err != nil {
			t.Fatalf("Failed to parse %d %q: %#v", i, update.Text, err)
		}

		fnPtr := reflect.ValueOf(state).Pointer()
		fnName := strings.Split(runtime.FuncForPC(fnPtr).Name(), ".")[2]
		fmt.Println(i, "STATE BEFORE:", fnName)

		state = state(msg)
		if state == nil && i < len(updates)-1 {
			t.Fatalf("Premature exit, state was already nil although there were updates in the pipe")
		}

		fnPtr = reflect.ValueOf(state).Pointer()
		if fnPtr != 0 {
			fnName = strings.Split(runtime.FuncForPC(fnPtr).Name(), ".")[2]
			fmt.Println(i, "STATE AFTER:", fnName)
		} else {
			fmt.Println(i, "STATE AFTER: nil")
		}
	}

	expectedMessages := []string{
		fmt.Sprintf("User Santana started a new game, join by using command: %s %s",
			JukeboxJuryPrefix, CommandJoin),
		"User Jesus joined the game",
		"User Santana wants to proceed, continuing...",
		"Add song with the following command and format in private chat with the bot: (esitä|esitys) " +
			"description here https://link-as-last-item",
		"Add review similar way (max score is 5, only integers): (arvio|arvioi|arvostele) description here 5/5",
		"Panelist Santana added a song",
		"Panelist Jesus added a song",
		"All songs submitted, continuing...",
		// First song introduction & review
		"The next song comes from the panelist Santana and the song's details: Description: " +
			"My favourite song, URL: https://example.com/satan_you_rock",
		"Panelist Jesus reviewed the song",
		"Everybody has reviewed the song, continuing...",
		"Jesus wrote: Great song1. The song rating was: 10/10",
		"Eventually the song https://example.com/satan_you_rock ended up catching 10.00 points",
		// Second song introduction & review
		"The next song comes from the panelist Jesus and the song's details: Description: Hallelujah 🤘, " +
			"URL: https://example.com/hesus",
		"Panelist Santana reviewed the song",
		"Everybody has reviewed the song, continuing...",
		"Santana wrote: Terrible song1. The song rating was: 1/10",
		"Eventually the song https://example.com/hesus ended up catching 1.00 points",
		"State: Ending the game",
		"Game has ended. The winner song came from Santana and was https://example.com/satan_you_rock with " +
			"10.00 average score",
	}

	if len(expectedMessages) != len(mockBot.receivedMessages) {
		t.Fatalf("Length of the expected messages %d differs from received %d, diff:\n%s\n",
			len(expectedMessages),
			len(mockBot.receivedMessages),
			cmp.Diff(expectedMessages, mockBot.receivedMessages),
		)
	}

	for i := range expectedMessages {
		if mockBot.receivedMessages[i] != expectedMessages[i] {
			t.Fatalf("Message index=%d, expected=%q, got=%q",
				i, mockBot.receivedMessages[i], expectedMessages[i],
			)
		}
	}
}

func TestNewMessage(t *testing.T) {
	t.Helper()

	type args struct {
		u tgbotapi.Update
	}
	tests := []struct { //nolint:govet // I'm fine to tosh away a few bytes in tests
		name          string
		args          args
		want          Message
		expectedError error
	}{
		{
			name: "Join",
			args: args{
				u: tgbotapi.Update{
					Message: &tgbotapi.Message{
						Chat: &tgbotapi.Chat{ID: 1},
						Text: "levyraati liity",
						From: &tgbotapi.User{UserName: "User1"},
					},
				},
			},
			want: Message{
				ChatID:     1,
				Command:    "liity",
				Text:       "",
				PlayerName: "User1",
			},
			expectedError: nil,
		},
		{
			name: "Join, with extra trailing chars",
			args: args{
				u: tgbotapi.Update{
					Message: &tgbotapi.Message{
						Chat: &tgbotapi.Chat{ID: 2},
						Text: "levyraati liity asdfasdf",
						From: &tgbotapi.User{UserName: "User1"},
					},
				},
			},
			want: Message{
				ChatID:     2,
				Command:    "liity",
				Text:       "asdfasdf",
				PlayerName: "User1",
			},
			expectedError: nil,
		},
		{
			name: "Join, with even more extra trailing chars",
			args: args{
				u: tgbotapi.Update{
					Message: &tgbotapi.Message{
						Chat: &tgbotapi.Chat{ID: 3},
						Text: "levyraati liity asdfasdf 1234",
						From: &tgbotapi.User{UserName: "User1"},
					},
				},
			},
			want: Message{
				ChatID:     3,
				Command:    "liity",
				Text:       "asdfasdf 1234",
				PlayerName: "User1",
			},
			expectedError: nil,
		},
		{
			name: "Too short",
			args: args{
				u: tgbotapi.Update{
					Message: &tgbotapi.Message{
						Text: "levyraati",
						Chat: &tgbotapi.Chat{ID: 4},
						From: &tgbotapi.User{UserName: "User1"},
					},
				},
			},
			want:          Message{},
			expectedError: ErrInvaliSyntax,
		},
		{
			name: "Invalid prefix",
			args: args{
				u: tgbotapi.Update{
					Message: &tgbotapi.Message{
						Chat: &tgbotapi.Chat{ID: 5},
						Text: "do something",
						From: &tgbotapi.User{UserName: "User1"},
					},
				},
			},
			want:          Message{},
			expectedError: ErrInvaliSyntax,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseToMessage(tt.args.u)
			if (err != nil) && !errors.Is(err, tt.expectedError) {
				t.Errorf("NewMessage() got error=%v, expected error=%v", err, tt.expectedError)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}
