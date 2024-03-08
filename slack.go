package main

import (
	"errors"
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func ConnectSlack() {
	defer App.wg.Done()

	api := slack.New(
		App.config.SlackBotToken,
		slack.OptionLog(log.Default().WithPrefix("slack api").StandardLog()),
		slack.OptionAppLevelToken(App.config.SlackAppToken),
	)

	resp, err := api.AuthTest()
	if err != nil {
		log.Error("Could not connect to Slack API.", "err", err)
		return
	}
	App.myUserId = resp.UserID

	App.slack = socketmode.New(
		api,
		socketmode.OptionLog(log.Default().WithPrefix("slack client").StandardLog()),
	)

	socketmodeHandler := socketmode.NewSocketmodeHandler(App.slack)
	socketmodeHandler.HandleEvents(slackevents.Message, handleMessage)
	socketmodeHandler.HandleSlashCommand("/buzerator", handleCommand)

	socketmodeHandler.Handle(socketmode.EventTypeConnecting, handleConnecting)
	socketmodeHandler.Handle(socketmode.EventTypeConnected, handleConnected)
	socketmodeHandler.Handle(socketmode.EventTypeHello, handleHello)
	socketmodeHandler.Handle(socketmode.EventTypeIncomingError, handleError)
	socketmodeHandler.Handle(socketmode.EventTypeConnectionError, handleConnError)

	err = socketmodeHandler.RunEventLoop()
	if err != nil {
		log.Fatalf("Error while running event loop: %v\n", err)
	}
}

func handleConnecting(evt *socketmode.Event, client *socketmode.Client) {
	log.Debug("Connecting to Slack...")
}

func handleConnected(evt *socketmode.Event, client *socketmode.Client) {
	log.Info("Connected to Slack.")
}

func handleHello(evt *socketmode.Event, client *socketmode.Client) {
	log.Debug("Received 'hello' from Slack.")
}

func handleError(evt *socketmode.Event, client *socketmode.Client) {
	ev, ok := evt.Data.(slack.IncomingEventError)
	if !ok {
		log.Warn("Invalid event data.", "evt", *evt)
		return
	}
	log.Error("Incoming error from Slack.", "err", ev.Error())
}

func handleConnError(evt *socketmode.Event, client *socketmode.Client) {
	ev, ok := evt.Data.(slack.ConnectionErrorEvent)
	if !ok {
		log.Warn("Invalid event data.", "evt", *evt)
		return
	}
	log.Error("Connection error from Slack.", "err", ev.Error())
}

func handleMessage(evt *socketmode.Event, client *socketmode.Client) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		log.Warn("Invalid event data.", "evt", *evt)
		return
	}
	client.Ack(*evt.Request)

	ev, ok := eventsAPIEvent.InnerEvent.Data.(*slackevents.MessageEvent)
	if !ok {
		log.Warn("Invalid event data.", "ev", *ev)
		return
	}

	logger := log.With("channel", ev.Channel, "ts", ev.TimeStamp)

	if ev.User == App.myUserId {
		logger.Debug("Ignoring message from myself.")
		return
	}

	if ev.ThreadTimeStamp == "" {
		logger.Debug("Ignoring non-thread message.")
		return
	}

	qi, err := LoadQuestionInstance(ev.Channel, ev.ThreadTimeStamp)
	if err != nil {
		logger.Error("Could not load question instance.", "err", err)
		return
	}

	if qi.QuestionID == 0 {
		logger.Debug("Ignoring reply to unrelated thread.")
		return
	}

	err = qi.HandleMessage(ev.User, ev.TimeStamp)
	if err != nil {
		logger.Error("Error while handling reply.", "err", err, "channel")
	}
}

func handleCommand(evt *socketmode.Event, client *socketmode.Client) {
	ev, ok := evt.Data.(slack.SlashCommand)
	if !ok {
		log.Warn("Invalid event data.", "evt", *evt)
		return
	}
	client.Ack(*evt.Request)

	token := App.webUI.CreateToken(ev.ChannelID)
	msg := fmt.Sprintf("Nastavenia tohto kanálu nájdeš tu: %s/%s/%s/", App.config.RootURL, ev.ChannelID, token)
	_, err := client.PostEphemeral(ev.ChannelID, ev.UserID, slack.MsgOptionText(msg, false))
	if err != nil {
		var slackErr slack.SlackErrorResponse
		ok := errors.As(err, &slackErr)
		
		if ok && (slackErr.Err == "channel_not_found" || slackErr.Err == "not_in_channel") {
			log.Warn("Received command from a channel I am not in.", "channel", ev.ChannelID, "user", ev.UserID)
			_, _, err := client.PostMessage(ev.UserID, slack.MsgOptionText("⚠️ Predtým, ako môžeš použiť `/buzerator` v nejakom kanáli, musíš ma doňho pridať.", false))
			if err != nil {
				log.Error("Could not send command not_in_channel notice.", "channel", ev.ChannelID, "user", ev.UserID)
			}
			return
		}

		log.Error("Could not send command reply.", "err", err)
	}
}

func ListChannelMembers(channel string) ([]string, error) {
	var allUsers []string
	cursor := "_initial"

	for cursor != "" {
		var response []string
		var err error

		if cursor == "_initial" {
			cursor = ""
		}

		response, cursor, err = App.slack.GetUsersInConversation(&slack.GetUsersInConversationParameters{
			ChannelID: channel,
			Cursor:    cursor,
		})
		if err != nil {
			return []string{}, err
		}
		allUsers = append(allUsers, response...)
	}

	return allUsers, nil
}

func LoadMemberName(user string) (string, error) {
	resp, err := App.slack.GetUserInfo(user)
	if err != nil {
		return "", err
	}
	return resp.RealName, err
}
