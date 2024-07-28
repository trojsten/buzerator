package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"

	"github.com/slack-go/slack"
	bolt "go.etcd.io/bbolt"
)

var Greetings = []string{
	"Ahojte!",
	"Zdravíčko!",
	"Nazdar!",
	"Bonjour!",
	"Som späť.",
	"Je tu nové číslo týždenníka FAKTY.",
	"Znovu nastal môj čas.",
	"Je čas na náš pravidelný update.",
	"Long time no see.",
}

type QuestionInstance struct {
	Question    *Question `json:"-"`
	QuestionID  uint64
	Timestamp   string
	LastMessage string
	Responses   map[string]bool
	Greeting    string
}

func (qi *QuestionInstance) Message() string {
	if qi.Greeting == "" {
		qi.Greeting = Greetings[rand.Intn(len(Greetings))]
	}

	var message []string
	message = append(message, fmt.Sprintf("👋 %s", qi.Greeting))
	message = append(message, "")
	message = append(message, fmt.Sprintf("> %s", strings.ReplaceAll(qi.Question.Message, "\n", "\n> ")))
	message = append(message, "")

	var usersOk, usersMissing []string
	for user, ok := range qi.Responses {
		mention := fmt.Sprintf("<@%s>", user)
		if ok {
			usersOk = append(usersOk, mention)
		} else {
			usersMissing = append(usersMissing, mention)
		}
	}

	if len(usersMissing) != 0 {
		message = append(message, "_Napíšte za seba update do threadu._")
		message = append(message, fmt.Sprintf("❌: %s", strings.Join(usersMissing, ", ")))
		message = append(message, fmt.Sprintf("✅: %s", strings.Join(usersOk, ", ")))
	} else {
		message = append(message, "🎉 Všetci už napísali svoj update, weeee!")
	}

	return strings.Join(message, "\n")
}

func (qi *QuestionInstance) PostMessage() error {
	message := qi.Message()
	client, ok := App.slack[qi.Question.TeamID]
	if !ok {
		return fmt.Errorf("not connected to team %s", qi.Question.TeamID)
	}

	if qi.Timestamp == "" {
		_, ts, err := client.PostMessage(qi.Question.Channel, slack.MsgOptionText(message, false))
		if err != nil {
			return err
		}

		qi.Timestamp = ts
	} else {
		_, _, _, err := client.UpdateMessage(qi.Question.Channel, qi.Timestamp, slack.MsgOptionText(message, false))
		if err != nil {
			return err
		}
	}

	return nil
}

func (qi *QuestionInstance) HandleMessage(user string, timestamp string) error {
	alreadyReplied, expected := qi.Responses[user]
	if !expected || alreadyReplied {
		qi.LastMessage = timestamp
		return qi.Save()
	}

	qi.Responses[user] = true
	qi.LastMessage = timestamp
	err := qi.Save()
	if err != nil {
		return err
	}

	err = qi.PostMessage()
	if err != nil {
		return err
	}

	client, ok := App.slack[qi.Question.TeamID]
	if !ok {
		return fmt.Errorf("not connected to team %s", qi.Question.TeamID)
	}
	_, err = client.PostEphemeral(qi.Question.Channel, user, slack.MsgOptionText("Ďakujem! ❤️", false), slack.MsgOptionTS(qi.Timestamp))
	return err
}

func (qi *QuestionInstance) CheckNewMessages() error {
	cursor := ""
	hasMore := true

	client, ok := App.slack[qi.Question.TeamID]
	if !ok {
		return fmt.Errorf("not connected to team %s", qi.Question.TeamID)
	}

	for hasMore {
		var messages []slack.Message
		var err error
		messages, hasMore, cursor, err = client.GetConversationReplies(&slack.GetConversationRepliesParameters{
			ChannelID: qi.Question.Channel,
			Timestamp: qi.Timestamp,
			Cursor:    cursor,
			Oldest:    qi.LastMessage,
		})

		if err != nil {
			return err
		}

		for _, message := range messages {
			err := qi.HandleMessage(message.User, message.Timestamp)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (qi *QuestionInstance) dbKey() []byte {
	return []byte(fmt.Sprintf("%s:%s", qi.Question.Channel, qi.Timestamp))
}

func (qi *QuestionInstance) Save() error {
	return App.db.Update(func(tx *bolt.Tx) error {
		messages := tx.Bucket([]byte("messages"))
		data, err := json.Marshal(qi)
		if err != nil {
			return err
		}

		return messages.Put(qi.dbKey(), data)
	})
}

func (qi *QuestionInstance) Delete() error {
	return App.db.Update(func(tx *bolt.Tx) error {
		messages := tx.Bucket([]byte("messages"))
		return messages.Delete(qi.dbKey())
	})
}

func (qi *QuestionInstance) LoadQuestion() error {
	q, err := LoadQuestion(qi.QuestionID)
	if err != nil {
		return err
	}
	qi.Question = &q
	return nil
}

func LoadQuestionInstance(channel string, timestamp string) (QuestionInstance, error) {
	key := []byte(fmt.Sprintf("%s:%s", channel, timestamp))
	var qi QuestionInstance

	err := App.db.View(func(tx *bolt.Tx) error {
		data := tx.Bucket([]byte("messages")).Get(key)
		if data == nil {
			return nil
		}

		return json.Unmarshal(data, &qi)
	})
	if err != nil {
		return qi, err
	}

	err = qi.LoadQuestion()
	return qi, err
}
