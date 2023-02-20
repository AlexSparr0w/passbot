package main

import (
	"database/sql"
	"encoding/base64"
	"log"
	"strings"
	"unicode/utf8"

	_ "github.com/lib/pq"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

func main() {
	db, err := sql.Open("postgres", "postgres://bot:password@db:5432/passbot?sslmode=disable")
	if err != nil {
		log.Panic(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Panic(err)
	}

	bot, err := tgbotapi.NewBotAPI("6045450017:AAGLWRnptfEb2BR34FGfa2u-n-RRhcCqEGo")
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "save":
				parts := strings.SplitN(update.Message.Text, " ", 3)
				if len(parts) != 3 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Использование: /save [имя ресурса] [пароль]. Например, /save google 123456"))
					continue
				}

				resource := parts[1]
				password := parts[2]

				encodedPassword := base64.StdEncoding.EncodeToString([]byte(password))

				_, err := db.Exec("INSERT INTO passwords (user_id, resource, password) VALUES ($1, $2, $3)", update.Message.From.ID, resource, encodedPassword)
				if err != nil {
					log.Panic(err)
				}
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Пароль успешно сохранен!"))

			case "list":
				rows, err := db.Query("SELECT resource FROM passwords WHERE user_id = $1", update.Message.From.ID)
				if err != nil {
					log.Panic(err)
				}
				defer rows.Close()

				var resources []string
				for rows.Next() {
					var resource string
					err := rows.Scan(&resource)
					if err != nil {
						log.Panic(err)
					}
					resources = append(resources, resource)
				}

				if len(resources) == 0 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "У вас нет сохраненных ресурсов."))
					continue
				}

				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Ваши сохраненные ресурсы:\n"+strings.Join(resources, "\n")))

			case "update":
				parts := strings.SplitN(update.Message.Text, " ", 4)
				if len(parts) != 4 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Использование: /update [имя ресурса] [старый пароль] [новый пароль]. Например, /update google 123456 654321"))
					continue
				}

				resource := parts[1]
				oldPassword := parts[2]
				newPassword := parts[3]

				var password string
				err := db.QueryRow("SELECT password FROM passwords WHERE user_id = $1 AND resource = $2", update.Message.From.ID, resource).Scan(&password)
				if err != nil {
					if err == sql.ErrNoRows {
						bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Пароль для указанного ресурса не найден."))
					} else {
						log.Panic(err)
					}
					continue
				}

				decodedPassword, err := base64.StdEncoding.DecodeString(password)
				if err != nil {
					log.Panic(err)
				}
				decodedPasswordStr := string(decodedPassword)
				if !utf8.ValidString(decodedPasswordStr) {
					decodedPasswordStr = string([]rune(decodedPasswordStr))
				}

				if decodedPasswordStr != oldPassword {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Неверный старый пароль."))
					continue
				}

				encodedNewPassword := base64.StdEncoding.EncodeToString([]byte(newPassword))
				_, err = db.Exec("UPDATE passwords SET password = $1 WHERE user_id = $2 AND resource = $3", encodedNewPassword, update.Message.From.ID, resource)
				if err != nil {
					log.Panic(err)
				}
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Пароль успешно обновлен!"))

			case "delete":
				parts := strings.SplitN(update.Message.Text, " ", 2)
				if len(parts) != 2 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Использование: /delete [имя ресурса]. Например, /delete google"))
					continue
				}

				resource := parts[1]

				_, err := db.Exec("DELETE FROM passwords WHERE user_id = $1 AND resource = $2", update.Message.From.ID, resource)
				if err != nil {
					log.Panic(err)
				}

				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Пароль для указанного ресурса успешно удален."))

			case "get":
				parts := strings.SplitN(update.Message.Text, " ", 2)
				if len(parts) != 2 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Использование: /get [имя ресурса]. Например, /get google"))
					continue
				}

				resource := parts[1]
				var password string
				err := db.QueryRow("SELECT password FROM passwords WHERE user_id = $1 AND resource = $2", update.Message.From.ID, resource).Scan(&password)
				if err != nil {
					if err == sql.ErrNoRows {
						bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Пароль для указанного ресурса не найден."))
					} else {
						log.Panic(err)
					}
					continue
				}

				decodedPassword, err := base64.StdEncoding.DecodeString(password)
				if err != nil {
					log.Panic(err)
				}
				decodedPasswordStr := string(decodedPassword)
				if !utf8.ValidString(decodedPasswordStr) {
					decodedPasswordStr = string([]rune(decodedPasswordStr))
				}

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, decodedPasswordStr)
				msg.ParseMode = "HTML"
				bot.Send(msg)

			default:
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Неизвестная команда. Доступные команды: /save, /get."))
			}
		}
	}
}
