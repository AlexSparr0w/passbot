package main

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"
	"unicode/utf8"

	_ "github.com/lib/pq"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

func deleteMessageWithDelay(bot *tgbotapi.BotAPI, chatID int64, messageID int, duration time.Duration) {
	time.Sleep(duration)
	deleteMsg := tgbotapi.NewDeleteMessage(chatID, messageID)
	if _, err := bot.Send(deleteMsg); err != nil {
		log.Printf("Error sending delete message: %v", err)
	}
}

func main() {
	db, err := sql.Open("postgres", "database_url")
	if err != nil {
		log.Panic(err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Panic(err)
	}

	bot, err := tgbotapi.NewBotAPI("bot_token")
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
			case "start":
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID,
					"Привет!\nЯ менеджер паролей. Каждый пароль я надежно шифрую и сохраняю\nДоступные команды: /save, /list, /update, /delete, /generate, /get, /help"))

			case "save":
				parts := strings.SplitN(update.Message.Text, " ", 4)
				if len(parts) != 4 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID,
						"Использование: /save [resource name] [login] [password]. Для примера, /save google john.doe@gmail.com 123456"))
					continue
				}

				resource := parts[1]
				login := parts[2]
				password := parts[3]

				encodedPassword := base64.StdEncoding.EncodeToString([]byte(password))
				_, err := db.Exec("INSERT INTO passwords (user_id, resource, login, password) VALUES ($1, $2, $3, pgp_sym_encrypt($4, 'sekretkey'))",
					update.Message.From.ID, resource, login, []byte(encodedPassword))
				if err != nil {
					log.Panic(err)
				}

				// Create the success message and send it
				successMsg := tgbotapi.NewMessage(update.Message.Chat.ID, "Пароль сохранен успешно!")
				msg, err := bot.Send(successMsg)
				if err != nil {
					log.Printf("Ошибка отправки сообщения: %v", err)
				} else {
					// Delete the user's message and the bot's response message after 5 seconds
					go func() {
						time.Sleep(5 * time.Second)
						deleteUserMsg := tgbotapi.NewDeleteMessage(update.Message.Chat.ID, update.Message.MessageID)
						bot.Send(deleteUserMsg)
						deleteBotMsg := tgbotapi.NewDeleteMessage(update.Message.Chat.ID, msg.MessageID)
						bot.Send(deleteBotMsg)
					}()
				}

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
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "У вас нету сохраненных данных."))
					continue
				}

				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Ваши пароли:\n"+strings.Join(resources, "\n"))
				response, err := bot.Send(msg)
				if err != nil {
					log.Printf("Ошибка отправки сообщения: %v", err)
				} else {
					// Delete the user's message and the bot's response message after 5 seconds
					go func() {
						time.Sleep(5 * time.Second)
						deleteUserMsg := tgbotapi.NewDeleteMessage(update.Message.Chat.ID, update.Message.MessageID)
						bot.Send(deleteUserMsg)
						deleteBotMsg := tgbotapi.NewDeleteMessage(update.Message.Chat.ID, response.MessageID)
						bot.Send(deleteBotMsg)
					}()
				}

			case "update":
				parts := strings.SplitN(update.Message.Text, " ", 4)
				if len(parts) != 4 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Использование: /update [resource name] [old password] [new password]. Для примера, /update google 123456 654321"))
					continue
				}

				resource := parts[1]
				oldPassword := parts[2]
				newPassword := parts[3]

				var password string
				err := db.QueryRow("SELECT pgp_sym_decrypt(password, 'sekretkey') FROM passwords WHERE user_id = $1 AND resource = $2", update.Message.From.ID, resource).Scan(&password)
				if err != nil {
					if err == sql.ErrNoRows {
						bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Пароль для данного ресурса не найден."))
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
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Не правильный старый пароль."))
					continue
				}

				encodedNewPassword := base64.StdEncoding.EncodeToString([]byte(newPassword))
				_, err = db.Exec("UPDATE passwords SET password = pgp_sym_encrypt($1, 'sekretkey')WHERE user_id = $2 AND resource = $3", []byte(encodedNewPassword), update.Message.From.ID, resource)
				if err != nil {
					log.Panic(err)
				}

				// Create the response message and send it
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Пароль был успешно обновлен!")
				response, err := bot.Send(msg)
				if err != nil {
					log.Printf("Ошибка отправки сообщения: %v", err)
				} else {
					// Delete the user's message and the bot's response message after 5 seconds
					go func() {
						time.Sleep(5 * time.Second)
						deleteUserMsg := tgbotapi.NewDeleteMessage(update.Message.Chat.ID, update.Message.MessageID)
						bot.Send(deleteUserMsg)
						deleteBotMsg := tgbotapi.NewDeleteMessage(update.Message.Chat.ID, response.MessageID)
						bot.Send(deleteBotMsg)
					}()
				}

			case "delete":
				parts := strings.SplitN(update.Message.Text, " ", 2)
				if len(parts) != 2 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Использование: /delete [resource name]. Для примера, /delete google"))
					continue
				}

				resource := parts[1]

				_, err := db.Exec("DELETE FROM passwords WHERE user_id = $1 AND resource = $2", update.Message.From.ID, resource)
				if err != nil {
					log.Panic(err)
				}

				// Create the response message and send it
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Пароль бы успешно удален!")
				response, err := bot.Send(msg)
				if err != nil {
					log.Printf("Ошибка отправки сообщения: %v", err)
				} else {
					// Delete the user's message and the bot's response message after 5 seconds
					go func() {
						time.Sleep(5 * time.Second)
						deleteUserMsg := tgbotapi.NewDeleteMessage(update.Message.Chat.ID, update.Message.MessageID)
						bot.Send(deleteUserMsg)
						deleteBotMsg := tgbotapi.NewDeleteMessage(update.Message.Chat.ID, response.MessageID)
						bot.Send(deleteBotMsg)
					}()
				}

			case "generate":
				var passwordLength int
				_, err := fmt.Sscanf(update.Message.Text, "generate %d", &passwordLength)
				if err != nil || passwordLength < 10 || passwordLength > 16 {
					passwordLength = 12
				}

				rand.Seed(time.Now().UnixNano())
				chars := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()")
				b := make([]rune, passwordLength)
				for i := range b {
					b[i] = chars[rand.Intn(len(chars))]
				}
				password := string(b)

				// Create the response message and send it
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, password)
				response, err := bot.Send(msg)
				if err != nil {
					log.Printf("Ошибка отправки сообщения: %v", err)
				} else {
					// Delete the user's message and the bot's response message after 5 seconds
					go func() {
						time.Sleep(5 * time.Second)
						deleteUserMsg := tgbotapi.NewDeleteMessage(update.Message.Chat.ID, update.Message.MessageID)
						bot.Send(deleteUserMsg)
						deleteBotMsg := tgbotapi.NewDeleteMessage(update.Message.Chat.ID, response.MessageID)
						bot.Send(deleteBotMsg)
					}()
				}

			case "get":
				parts := strings.SplitN(update.Message.Text, " ", 2)
				if len(parts) != 2 {
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Использование: /get [resource name]. Для примера, /get google"))
					continue
				}
				resource := parts[1]
				var encodedPassword string
				var login string
				err := db.QueryRow("SELECT pgp_sym_decrypt(password, 'sekretkey'), login FROM passwords WHERE user_id = $1 AND resource = $2", update.Message.From.ID, resource).Scan(&encodedPassword, &login)
				if err != nil {
					if err == sql.ErrNoRows {
						bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Пароль для данного ресурса не найден."))
					} else {
						log.Panic(err)
					}
					continue
				}

				decodedPassword, err := base64.StdEncoding.DecodeString(encodedPassword)
				if err != nil {
					log.Panic(err)
				}
				decodedPasswordStr := string(decodedPassword)
				if !utf8.ValidString(decodedPasswordStr) {
					decodedPasswordStr = string([]rune(decodedPasswordStr))
				}

				// Create the response message and send it
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("<b>Login:</b> %s\n<b>Password:</b> %s", login, decodedPasswordStr))
				msg.ParseMode = "HTML"
				response, err := bot.Send(msg)
				if err != nil {
					log.Printf("Ошибка отправки сообщения: %v", err)
				} else {
					// Delete the user's message and the bot's response message after 5 seconds
					go func() {
						time.Sleep(5 * time.Second)
						deleteUserMsg := tgbotapi.NewDeleteMessage(update.Message.Chat.ID, update.Message.MessageID)
						bot.Send(deleteUserMsg)
						deleteBotMsg := tgbotapi.NewDeleteMessage(update.Message.Chat.ID, response.MessageID)
						bot.Send(deleteBotMsg)
					}()
				}
			case "help":
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Доступания команда: \n/save [resource name] [login] [password]\n/list\n/update [resource name] [old password] [new password]\n/delete [resource name]\n/generate\n/get [resource name]"))
			case "default":
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Неизвестная команда. Доступания команда: /save, /list, /update, /delete, /generate, /get"))
			}
		}
	}
}
