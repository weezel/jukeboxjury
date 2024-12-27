package main

import (
	"flag"
	"os"
	"path/filepath"
	"strconv"

	"weezel/jukeboxjury/internal/game"
	"weezel/jukeboxjury/internal/logger"

	"github.com/joho/godotenv"
	_ "github.com/joho/godotenv/autoload"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var flagConfigFile string

func loadConfigFile() {
	flag.StringVar(&flagConfigFile, "f", "", "Default config file location")
	flag.Parse()

	configFileAbs, err := filepath.Abs(flagConfigFile)
	if err != nil {
		logger.Logger.Fatal().Err(err).Msg("Cannot get absolute path of config file")
	}

	if err = godotenv.Load(configFileAbs); err != nil {
		logger.Logger.Fatal().Err(err).Msg("Cannot load config file")
	}

	logger.Logger.Info().Msgf("Config file %s succesfully loaded", configFileAbs)
}

func main() {
	loadConfigFile()

	tgramAPI, err := tgbotapi.NewBotAPI(os.Getenv("BOT_API_TOKEN"))
	if err != nil {
		logger.Logger.Fatal().Err(err).Msg("Failed to create new bot")
	}

	chatID, err := strconv.ParseInt(os.Getenv("CHAT_ID"), 10, 64)
	if err != nil {
		logger.Logger.Fatal().Err(err).Msg("Invalid chat ID")
	}

	resultsDir := os.Getenv("RESULTS_DIRECTORY")
	p := game.New(
		tgramAPI,
		chatID,
		game.WithOutputDirectory(&resultsDir),
		game.WithResultsURL(os.Getenv("RESULTS_URL")),
	)
	state := p.StartGame

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	logger.Logger.Info().Msg("Waiting for messages...")
	for update := range tgramAPI.GetUpdatesChan(u) {
		if update.Message == nil {
			continue
		}

		msg, err := game.ParseToMessage(tgbotapi.Update{Message: update.Message})
		if err != nil {
			logger.Logger.Debug().Err(err).
				Interface("payload", update.Message.Text).
				Msg("Failed to parse message")
			continue
		}

		state = state(msg)
		if state == nil {
			logger.Logger.Info().Msg("All good, getting back to the init state")
			state = p.StartGame
			continue
		}
	}
}
