package bot

import (
	"log"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/jasonlvhit/gocron"
	"go.uber.org/zap"

	"github.com/ErrorBoi/zhirobot/db"
)

// Bot unites botAPI and channels
type Bot struct {
	BotAPI *tgbotapi.BotAPI
	DB     *db.DB
	lg     *zap.SugaredLogger
	ChatID int64
}

// InitBot inits a bot with given Token
func InitBot(BotToken string, DB *db.DB, lg *zap.SugaredLogger) (*Bot, error) {
	var err error
	botAPI, err := tgbotapi.NewBotAPI(BotToken)
	if err != nil {
		return nil, err
	}

	return &Bot{
		BotAPI: botAPI,
		DB:     DB,
		lg:     lg,
		ChatID: ZhirosbrosChatID,
	}, nil
}

// InitUpdates inits an Updates Channel
func (b *Bot) InitUpdates(BotToken string) {
	ucfg := tgbotapi.NewUpdate(0)
	ucfg.Timeout = 60

	updates := b.BotAPI.ListenForWebhook("/" + BotToken)
	log.Printf("Authorized on account %s", b.BotAPI.Self.UserName)

	go b.RunScheduler()

	for update := range updates {
		if update.Message == nil { // ignore any non-Message Updates
			continue
		}

		if update.Message.IsCommand() {
			b.ExecuteCommand(update.Message)
		} else {
			b.ExecuteText(update.Message)
		}

		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)
	}
}

// ExecuteCommand distributes commands to go routines
func (b *Bot) ExecuteCommand(m *tgbotapi.Message) {
	command := strings.ToLower(m.Command())

	switch command {
	case "faq":
		go b.faq(m)
	case "start":
		go b.start(m)
	case "help":
		go b.help(m)
	case "setweight", "sw":
		go b.setWeight(m)
	case "getweight", "gw":
		go b.getWeight(m)
	case "invite":
		go b.getInviteLink(m)
	case "on":
		go b.turnNotifyOn(m)
	case "off":
		go b.turnNotifyOff(m)
	default:
		if m.Chat.IsPrivate() {
			msg := tgbotapi.NewMessage(m.Chat.ID, "Я не знаю такой команды (凸ಠ益ಠ)凸\nНапиши /help и получи справку по командам")
			msg.ReplyToMessageID = m.MessageID
			b.BotAPI.Send(msg)
		}
	}
}

// ExecuteText parses user weight from non-command messages and sends it to database
func (b *Bot) ExecuteText(m *tgbotapi.Message) {
	if m.Chat.IsPrivate() {
		b.parseAndSet(m, m.Text)
	}
}

func (b *Bot) RunScheduler() {
	loc, err := time.LoadLocation("Europe/Moscow")
	if err != nil {
		b.lg.Errorf("Load time location error: %w", err)
	}

	gocron.ChangeLoc(loc)

	// Send "Time to weigh" reminder every Sunday
	gocron.Every(1).Sunday().At("11:00").Do(b.weeklyNotification)

	// Wake Up a bot before it goes to idling
	gocron.Every(15).Minute().Do(b.wakeUp)

	// Start all the pending jobs
	<- gocron.Start()
}
