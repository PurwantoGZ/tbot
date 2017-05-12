package adapter

import (
	"fmt"
	"log"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type BotAdapter interface {
	Send(*Message) error
	GetUpdatesChan(string, string) (<-chan *Message, error)
	GetUserName() string
	GetFirstName() string
}

type Bot struct {
	tbot *tgbotapi.BotAPI
}

func CreateBot(token string) (BotAdapter, error) {
	tbot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}
	return &Bot{tbot: tbot}, nil
}

func (b *Bot) Send(m *Message) error {
	c := chattableFromMessage(m)
	if c != nil {
		_, err := b.tbot.Send(c)
		return err
	}
	return fmt.Errorf("Trying to send nil chattable. Message: %v", m)
}

func (b *Bot) GetUpdatesChan(webhookURL string, listenAddr string) (<-chan *Message, error) {
	messages := make(chan *Message)
	var updates <-chan tgbotapi.Update
	var err error
	if webhookURL == "" {
		u := tgbotapi.NewUpdate(0)
		u.Timeout = 60
		updates, err = b.tbot.GetUpdatesChan(u)
		if err != nil {
			return nil, err
		}
	} else {
		config := tgbotapi.NewWebhook(webhookURL)
		b.tbot.SetWebhook(config)
		updates = b.tbot.ListenForWebhook("/")
		go http.ListenAndServe(listenAddr, nil)
	}
	go b.adaptUpdates(updates, messages)
	return messages, nil
}

func (b *Bot) GetUserName() string {
	return b.tbot.Self.UserName
}

func (b *Bot) GetFirstName() string {
	return b.tbot.Self.FirstName
}

func (b *Bot) adaptUpdates(updates <-chan tgbotapi.Update, messages chan<- *Message) {
	var err error
	for update := range updates {
		message := &Message{
			Replies: make(chan *Message),
			From:    update.Message.From.UserName,
			ChatID:  update.Message.Chat.ID,
		}
		switch {
		case update.Message.Document != nil:
			message.Data, err = b.tbot.GetFileDirectURL(update.Message.Document.FileID)
			if err != nil {
				log.Println(err)
				continue
			}
			message.Type = MessageDocument
			messages <- message
		case update.Message.Text != "":
			message.Type = MessageText
			message.Data = update.Message.Text
			messages <- message
		}
	}
}

func chattableFromMessage(m *Message) tgbotapi.Chattable {
	switch m.Type {
	case MessageText:
		return tgbotapi.NewMessage(m.ChatID, m.Data)
	case MessageSticker:
		return tgbotapi.NewStickerUpload(m.ChatID, m.Data)
	case MessagePhoto:
		photo := tgbotapi.NewPhotoUpload(m.ChatID, m.Data)
		photo.Caption = m.Caption
		return photo
	case MessageAudio:
		return tgbotapi.NewAudioUpload(m.ChatID, m.Data)
	case MessageDocument:
		return tgbotapi.NewDocumentUpload(m.ChatID, m.Data)
	case MessageKeyboard:
		msg := tgbotapi.NewMessage(m.ChatID, m.Data)
		btns := buttonsFromStrings(m.Buttons)
		msg.ReplyMarkup = tgbotapi.NewReplyKeyboard(btns...)
		return msg
	}
	return nil
}

func buttonsFromStrings(strs [][]string) [][]tgbotapi.KeyboardButton {
	btns := make([][]tgbotapi.KeyboardButton, len(strs))
	for i, buttonRow := range strs {
		btns[i] = make([]tgbotapi.KeyboardButton, len(buttonRow))
		for j, buttonText := range buttonRow {
			btns[i][j] = tgbotapi.NewKeyboardButton(buttonText)
		}
	}
	return btns
}
