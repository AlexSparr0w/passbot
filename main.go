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
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Usage: /save [resource name] [password]. For example, /save google 123456"))
					continue
				}

				resource := parts[1]
				password := parts[2]

				encodedPassword := base64.StdEncoding.EncodeToString([]byte(password))

				_, err := db.Exec("INSERT INTO passwords (user_id, resource, password) VALUES ($1, $2, $3)", update.Message.From.ID, resource, encodedPassword)
				if err != nil {
					log.Panic(err)
				}

				// Create the success message and send it
				successMsg := tgbotapi.NewMessage(update.Message.Chat.ID, "Password saved successfully!")
				msg, err := bot.Send(successMsg)
				if err != nil {
					log.Printf("Error sending message: %v", err)
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
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "You don't have any saved resources."))
					continue
				}

				// Create the response message and send it
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Your saved resources:\n"+strings.Join(resources, "\n"))
				response, err := bot.Send(msg)
				if err != nil {
					log.Printf("Error sending message: %v", err)
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
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Usage: /update [resource name] [old password] [new password]. For example, /update google 123456 654321"))
					continue
				}

				resource := parts[1]
				oldPassword := parts[2]
				newPassword := parts[3]

				var password string
				err := db.QueryRow("SELECT password FROM passwords WHERE user_id = $1 AND resource = $2", update.Message.From.ID, resource).Scan(&password)
				if err != nil {
					if err == sql.ErrNoRows {
						bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Password for the specified resource is not found."))
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
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Incorrect old password."))
					continue
				}

				encodedNewPassword := base64.StdEncoding.EncodeToString([]byte(newPassword))
				_, err = db.Exec("UPDATE passwords SET password = $1 WHERE user_id = $2 AND resource = $3", encodedNewPassword, update.Message.From.ID, resource)
				if err != nil {
					log.Panic(err)
				}

				// Create the response message and send it
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Password for the specified resource has been successfully updated.")
				response, err := bot.Send(msg)
				if err != nil {
					log.Printf("Error sending message: %v", err)
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
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Usage: /delete [resource name]. For example, /delete google"))
					continue
				}

				resource := parts[1]

				_, err := db.Exec("DELETE FROM passwords WHERE user_id = $1 AND resource = $2", update.Message.From.ID, resource)
				if err != nil {
					log.Panic(err)
				}

				// Create the response message and send it
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Password for the specified resource has been successfully deleted.")
				response, err := bot.Send(msg)
				if err != nil {
					log.Printf("Error sending message: %v", err)
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
					log.Printf("Error sending message: %v", err)
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
					bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Usage: /get [resource name]. For example, /get google"))
					continue
				}

				resource := parts[1]
				var password string
				err := db.QueryRow("SELECT password FROM passwords WHERE user_id = $1 AND resource = $2", update.Message.From.ID, resource).Scan(&password)
				if err != nil {
					if err == sql.ErrNoRows {
						bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Password for the specified resource was not found."))
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

				// Create the response message and send it
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, decodedPasswordStr)
				msg.ParseMode = "HTML"
				response, err := bot.Send(msg)
				if err != nil {
					log.Printf("Error sending message: %v", err)
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
			case "default":
				bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Unknown command. Available commands: /save, /list, /update, /delete, /generate, /get"))
			}
		}
	}
}
