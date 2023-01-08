package when_electricity_sumy

import (
	"encoding/json"
	"fmt"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Request struct {
	Queue int
	Day   time.Time
}

const pad = "      "

var keyboard = tgbotapi.NewReplyKeyboard(
	tgbotapi.NewKeyboardButtonRow(
		tgbotapi.NewKeyboardButton("1 черга"),
		tgbotapi.NewKeyboardButton("2 черга"),
		tgbotapi.NewKeyboardButton("3 черга"),
	),
)

var nonNumericRegex = regexp.MustCompile(`[^0-9]+`)

var schedules = map[string]map[int]map[int][][]int{
	"+2/-4": {
		1: {
			1: {{0, 2}, {4, 8}, {10, 14}, {16, 20}, {22, 0}},
			2: {{2, 6}, {8, 12}, {14, 18}, {20, 0}},
			0: {{2, 4}, {6, 10}, {12, 16}, {18, 22}},
		},
		2: {
			1: {{2, 4}, {6, 10}, {12, 16}, {18, 22}},
			2: {{0, 2}, {4, 8}, {10, 14}, {16, 20}, {22, 0}},
			0: {{0, 2}, {2, 6}, {8, 12}, {14, 18}, {20, 0}},
		},
		3: {
			1: {{2, 6}, {8, 12}, {14, 18}, {20, 0}},
			2: {{2, 4}, {6, 10}, {12, 16}, {18, 22}},
			0: {{0, 2}, {4, 8}, {10, 14}, {16, 20}, {22, 0}},
		},
	},
	"+4/-2": {
		1: {
			1: {{0, 2}, {6, 8}, {12, 14}, {18, 20}},
			2: {{4, 6}, {10, 12}, {16, 18}, {22, 0}},
			0: {{2, 4}, {8, 10}, {14, 16}, {20, 22}},
		},
		2: {
			1: {{2, 4}, {8, 10}, {14, 16}, {20, 22}},
			2: {{0, 2}, {6, 8}, {12, 14}, {18, 20}},
			0: {{4, 6}, {10, 12}, {16, 18}, {22, 0}},
		},
		3: {
			1: {{4, 6}, {10, 12}, {16, 18}, {22, 0}},
			2: {{2, 4}, {8, 10}, {14, 16}, {20, 22}},
			0: {{0, 2}, {6, 8}, {12, 14}, {18, 20}},
		},
	},
	"+24/-0": {
		1: {
			1: {},
			2: {},
			0: {},
		},
		2: {
			1: {},
			2: {},
			0: {},
		},
		3: {
			1: {},
			2: {},
			0: {},
		},
	},
}

func init() {
	functions.HTTP("process", process)
}

func process(w http.ResponseWriter, r *http.Request) {
	scheduleFormat := os.Getenv("APP_SCHEDULE")
	_, ok := schedules[scheduleFormat]
	if !ok {
		log.Panic("Wrong schedule format")
	}

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN"))
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = strings.ToLower(os.Getenv("APP_DEBUG")) == "true"

	info, err := bot.GetWebhookInfo()
	if err != nil {
		log.Panic(err)
	}

	if info.LastErrorDate != 0 {
		log.Printf("Telegram callback failed: %s", info.LastErrorMessage)
	}

	update, err := bot.HandleUpdate(r)
	if err != nil {
		log.Fatal(err)
	}

	var msg tgbotapi.MessageConfig

	if update.Message != nil && update.Message.IsCommand() {
		msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Оберіть чергу")
		msg.ReplyMarkup = keyboard
	} else if update.Message != nil {
		msg = handleDayRequest(update.Message)
	} else if update.CallbackQuery != nil {
		deleteMessage := tgbotapi.NewDeleteMessage(
			update.CallbackQuery.Message.Chat.ID,
			update.CallbackQuery.Message.MessageID,
		)
		bot.Send(deleteMessage)

		request := Request{}
		json.Unmarshal([]byte(update.CallbackQuery.Data), &request)
		day, _ := strconv.Atoi(request.Day.Format("2"))

		msg = tgbotapi.NewMessage(
			update.CallbackQuery.Message.Chat.ID,
			getSchedule(request, schedules[scheduleFormat][request.Queue][day%3]),
		)
		msg.ReplyMarkup = keyboard
	}

	if msg.Text == "" {
		msg = tgbotapi.NewMessage(update.Message.Chat.ID, "Я вас не зрозумів, оберіть чергу відключень електроенергії")
		msg.ReplyMarkup = keyboard
	}

	bot.Send(msg)

	return
}

func handleDayRequest(message *tgbotapi.Message) tgbotapi.MessageConfig {
	queue, _ := strconv.Atoi(nonNumericRegex.ReplaceAllString(message.Text, ""))

	if queue < 1 || queue > 3 {
		msg := tgbotapi.NewMessage(message.Chat.ID, "Оберіть чергу (поточний вибір: "+message.Text+")")
		msg.ReplyMarkup = keyboard

		return msg
	}

	loc, _ := time.LoadLocation("Europe/Kyiv")
	localTime := message.Time().In(loc)

	today, _ := json.Marshal(Request{Queue: queue, Day: localTime})
	y, m, d := localTime.AddDate(0, 0, 1).Date()
	tomorrow, _ := json.Marshal(Request{Queue: queue, Day: time.Date(y, m, d, 0, 0, 0, 0, loc)})

	msg := tgbotapi.NewMessage(message.Chat.ID, "Оберіть день")
	msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Сьогодні", string(today)),
			tgbotapi.NewInlineKeyboardButtonData("Завтра", string(tomorrow)),
		),
	)

	return msg
}

func getSchedule(request Request, schedule [][]int) string {
	hour, _ := strconv.Atoi(request.Day.Format("15"))
	msgText := ""

	for i := range schedule {
		if schedule[i][1] <= hour {
			continue
		}

		endTime := schedule[i][1]
		if endTime == 24 {
			endTime = 0
		}

		msgText += fmt.Sprintf("%s%02d:00 - %02d:00\n", pad, schedule[i][0], endTime)
	}

	if msgText == "" {
		msgText += pad + "🚫 Не заплановано"
	}

	return fmt.Sprintf(
		"⏰ Графік вимкнень електроенергії на %s з %d години для %d черги\n\n",
		request.Day.Format("02.01.2006"),
		hour,
		request.Queue,
	) + msgText + "\n"
}
