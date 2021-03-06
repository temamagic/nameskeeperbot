package main

import (
	"bytes"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"log"
	"strings"
	"time"
)

var db *redis.Client
var bot *tgbotapi.BotAPI

func main() {
	var err error

	db = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       6,
	})

	bot, err = tgbotapi.NewBotAPI("<TOKEN>")
	if err != nil {
		log.Fatalln("can't access Bot API: ", err)
	}

	log.Printf("Started listening on @%s\n", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}
		msg := update.Message
		go saveName(msg.From)
		if msg.Chat.IsPrivate(){
			if strings.ToLower(msg.Command()) == "start" {
				handleStart(msg)
				continue
			}
			if msg.ForwardFrom != nil {
				handleSearch(msg, msg.ForwardFrom.ID)
				continue
			}
			handleUsage(msg)
			continue
		}
		if strings.ToLower(msg.Command()) == "names" {
			if msg.ReplyToMessage == nil {
				handleUsage(msg)
				continue
			}
			handleSearch(msg, msg.ReplyToMessage.From.ID)
		}
	}
}

func handleStart(msg *tgbotapi.Message) {
	var buff bytes.Buffer
	buff.WriteString("Hey there! I'm Names Keeper.\n")
	buff.WriteString("I can show you one's name history.\n\n")
	buff.WriteString("There are two ways to ask me for that:\n")
	buff.WriteString("1. Forward me one's message privately\n")
	buff.WriteString("2. Reply to one's message with /names command while being in group\n\n")
	buff.WriteString("Please note that I learn names listening to groups so if I don't know one's history, he/she hasn't been chatting in group where I exist while changing names.")
	_, _ = bot.Send(tgbotapi.NewMessage(msg.Chat.ID, fmt.Sprintf(buff.String())))
}

func handleUsage(msg *tgbotapi.Message) {
	c := tgbotapi.NewMessage(msg.Chat.ID, "Hey, I do only work with forwards in private and with /names command replied on target user in groups")
	c.ReplyToMessageID = msg.MessageID
	_, _ = bot.Send(c)
}

func handleSearch(replyTo *tgbotapi.Message, targetID int) {
	msg := getNamesMessage(targetID)
	c := tgbotapi.NewMessage(replyTo.Chat.ID, msg)
	c.ReplyToMessageID = replyTo.MessageID
	_, _ = bot.Send(c)
}

func saveName(user *tgbotapi.User) {
	currentName := strings.TrimSpace(user.FirstName + " " + user.LastName)
	if user.UserName != "" {
		currentName += " @" + user.UserName
	}
	db.ZAdd(getUserKey(user.ID), redis.Z{
		Score: float64(time.Now().Unix()),
		Member: currentName,
	})
}

func getNamesMessage(userID int) (message string) {
	records := db.ZRevRangeByScoreWithScores(getUserKey(userID), redis.ZRangeBy{
		Min: "-inf",
		Max: "+inf",
	}).Val()
	if len(records) == 0 {
		return "I haven't learned any names of this user :(\nTry adding me to the group where he/she talks frequently."
	}
	for i, record := range records {
		lastSeen := "Last known"
		if i != 0 {
			lastSeen = "Until " + time.Unix(int64(record.Score), 0).Format("02.01.2006")
		}
		message += fmt.Sprintf("%s: %s\n", lastSeen, record.Member.(string))
	}
	return
}

func getUserKey(userID int) string {
	return fmt.Sprintf("user.%d", userID)
}