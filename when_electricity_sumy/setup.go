package when_electricity_sumy

import (
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"net/http"
	"os"
)

func init() {
	functions.HTTP("setup", setup)
}

func setup(w http.ResponseWriter, r *http.Request) {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN"))
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	wh, err := tgbotapi.NewWebhook(os.Getenv("HTTP_HOST") + "/" + bot.Token)
	if err != nil {
		log.Panic(err)
	}

	_, err = bot.Request(wh)
	if err != nil {
		log.Panic(err)
	}

	_, err = bot.GetWebhookInfo()
	if err != nil {
		log.Panic(err)
	}

	os.Exit(0)
}
