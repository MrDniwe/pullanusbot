package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	tb "gopkg.in/tucnak/telebot.v2"
)

// SMS structure
type SMS struct {
}

type smsRegGetBalance struct {
	Response string `json:"response"`
	Balance  string `json:"balance"`
}

// initialize database and all nesessary command handlers
func (s *SMS) initialize() {
	if os.Getenv("SMS_API_KEY") == "" {
		log.Println("SMS: SMS_API_KEY not set! Skipping...")
		return
	}

	if os.Getenv("ADMIN_CHAT_ID") == "" {
		log.Println("SMS: ADMIN_CHAT_ID not set! Skipping...")
		return
	}

	log.Println("SMS: database initialization")

	_, err := db.Exec("CREATE TABLE IF NOT EXISTS sms (chat_id INTEGER, enabled INTEGER, processed INTEGER)")
	checkErr(err)

	log.Println("VPN: subscribing to bot events")

	bot.Handle("/sms", s.start)

	log.Println("SMS: successfully initialized")
}

func (s *SMS) start(m *tb.Message) {

	// INSERT if not exist
	stmt, err := db.Prepare(`
	INSERT INTO sms(chat_id, enabled, processed) 
	SELECT ?, ?, ?
	WHERE NOT EXISTS(SELECT 1 FROM sms WHERE chat_id = ?);`)
	checkErr(err)
	defer stmt.Close()

	_, err = stmt.Exec(m.Chat.ID, 0, 0, m.Chat.ID)
	checkErr(err)

	var enabled, processed int
	err = db.QueryRow("SELECT enabled, processed FROM sms WHERE chat_id = ?", m.Chat.ID).Scan(&enabled, &processed)
	checkErr(err)

	log.Printf("SMS: chat_id: %d enabled: %d processed: %d", m.Chat.ID, enabled, processed)

	if processed == 0 {
		s.authorize(m)
		return
	}

	if enabled == 0 {
		bot.Send(m.Chat, i18n("sms_access_denied"))
		return // NOT AUTHORIZED!
	}

	resp, err := http.Get("http://api.sms-reg.com/getBalance.php?apikey=" + os.Getenv("SMS_API_KEY"))
	if err != nil {
		log.Printf("SMS: Balance request error: %s", err)
		// handle error
	}
	defer resp.Body.Close()

	var srResp smsRegGetBalance
	body, _ := ioutil.ReadAll(resp.Body)

	err = json.Unmarshal(body, &srResp)
	if err != nil {
		log.Printf("SMS: json parse error: %s", err)
		return
	}

	var msg *tb.Message
	getNumberBtn := tb.InlineButton{
		Unique: "get_phone_button",
		Text:   i18n("sms_get_phone_button"),
	}
	bot.Handle(&getNumberBtn, func(c *tb.Callback) {
		b, ok := bot.(*tb.Bot)
		if !ok {
			log.Println("SMS: Bot cast failed")
			return
		}
		b.Edit(msg, msg.Text, &tb.ReplyMarkup{InlineKeyboard: nil})
		b.Respond(c, &tb.CallbackResponse{Text: i18n("sms_phone_request_sent")})
		s.processPhone(m)
	})

	msg, _ = bot.Send(m.Chat, "💰 Balance: "+srResp.Balance, &tb.ReplyMarkup{
		InlineKeyboard: [][]tb.InlineButton{
			[]tb.InlineButton{getNumberBtn},
		},
	})
}

func (s *SMS) authorize(m *tb.Message) {
	b, ok := bot.(*tb.Bot)
	if !ok {
		log.Println("SMS: Bot cast failed")
		return
	}

	requestPermissionBtn := tb.InlineButton{
		Unique: "auth_button",
		Text:   i18n("sms_auth_button"),
	}

	b.Handle(&requestPermissionBtn, func(c *tb.Callback) {
		// on inline button pressed (callback!)
		var processed int
		err := db.QueryRow("SELECT processed FROM sms WHERE chat_id = ?", m.Chat.ID).Scan(&processed)
		checkErr(err)

		if processed == 1 {
			b.Respond(c, &tb.CallbackResponse{Text: i18n("sms_auth_already_processed")})
			return
		}

		accessGrantedBtn := tb.InlineButton{
			Unique: "access_granted",
			Text:   "✅ Authorize",
		}
		b.Handle(&accessGrantedBtn, func(c *tb.Callback) {
			stmt, err := db.Prepare("UPDATE sms SET enabled = ? , processed = ? WHERE chat_id = ?")
			checkErr(err)
			defer stmt.Close()

			_, err = stmt.Exec(1, 1, m.Chat.ID)
			checkErr(err)

			b.Respond(c, &tb.CallbackResponse{Text: "✅ Access granted!"})
			b.Send(m.Chat, i18n("sms_access_granted"))
		})

		accessDeniedBtn := tb.InlineButton{
			Unique: "access_denied",
			Text:   "❌ Revoke Access",
		}
		b.Handle(&accessDeniedBtn, func(c *tb.Callback) {
			stmt, err := db.Prepare("UPDATE sms SET enabled = ? , processed = ? WHERE chat_id = ?")
			checkErr(err)
			defer stmt.Close()

			_, err = stmt.Exec(0, 1, m.Chat.ID)
			checkErr(err)

			b.Respond(c, &tb.CallbackResponse{Text: "❌ Access denied!"})
			b.Send(m.Chat, i18n("sms_access_denied"))
		})

		b.Send(&Admin{}, fmt.Sprintf(i18n("sms_auth_request_message"), m.Chat.ID, m.Chat.Title, m.Chat.Type, m.Sender.FirstName, m.Sender.LastName, m.Sender.Username), &tb.ReplyMarkup{
			InlineKeyboard: [][]tb.InlineButton{
				[]tb.InlineButton{accessGrantedBtn},
				[]tb.InlineButton{accessDeniedBtn},
			},
		})
		// always respond!
		b.Respond(c, &tb.CallbackResponse{Text: i18n("sms_auth_request_sent")})
	})
	b.Send(m.Chat, i18n("sms_greeting"), &tb.ReplyMarkup{
		InlineKeyboard: [][]tb.InlineButton{
			[]tb.InlineButton{requestPermissionBtn},
		},
	})
}

func (s *SMS) processPhone(m *tb.Message) {
	// Do phone request
	var msg *tb.Message
	smsSentBtn := tb.InlineButton{
		Unique: "sms_sent_button",
		Text:   i18n("sms_sent_button"),
	}

	bot.Handle(&smsSentBtn, func(c *tb.Callback) {
		b, ok := bot.(*tb.Bot)
		if !ok {
			log.Println("SMS: Bot cast failed")
			return
		}
		b.Edit(msg, msg.Text, &tb.ReplyMarkup{InlineKeyboard: nil})
		b.Respond(c, &tb.CallbackResponse{})
		s.processSMS(m)
	})

	text := fmt.Sprintf(i18n("sms_phone_received"), "7XXXXXXXXXX")
	msg, _ = bot.Send(m.Chat, text, &tb.SendOptions{ParseMode: tb.ModeMarkdown}, &tb.ReplyMarkup{
		InlineKeyboard: [][]tb.InlineButton{
			[]tb.InlineButton{smsSentBtn},
		},
	})
}

func (s *SMS) processSMS(m *tb.Message) {

}
