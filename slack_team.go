package main

import (
	"errors"
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

type SlackTeamClient struct {
	TeamID    string
	BotUserID string
	client    *socketmode.Client
	log       *log.Logger
}

func ConnectTeam(team Team) error {
	logger := log.WithPrefix(fmt.Sprintf("[%s]", team.ID))
	logger.Debug("Connecting...")

	api := slack.New(
		team.Token,
		slack.OptionLog(log.Default().WithPrefix("slack api").StandardLog()),
		slack.OptionAppLevelToken(App.config.SlackAppToken),
	)

	teamClient := &SlackTeamClient{
		TeamID: team.ID,
		log:    logger,
	}

	resp, err := api.AuthTest()
	if err != nil {
		return err
	}
	teamClient.BotUserID = resp.UserID

	teamClient.client = socketmode.New(
		api,
		socketmode.OptionLog(log.Default().WithPrefix("slack client").StandardLog()),
	)
	App.slack[team.ID] = teamClient.client

	socketmodeHandler := socketmode.NewSocketmodeHandler(teamClient.client)
	socketmodeHandler.HandleEvents(slackevents.Message, teamClient.handleMessage)
	socketmodeHandler.HandleSlashCommand("/buzerator", teamClient.handleCommand)

	socketmodeHandler.Handle(socketmode.EventTypeConnecting, teamClient.handleConnecting)
	socketmodeHandler.Handle(socketmode.EventTypeConnected, teamClient.handleConnected)
	socketmodeHandler.Handle(socketmode.EventTypeHello, teamClient.handleHello)
	socketmodeHandler.Handle(socketmode.EventTypeIncomingError, teamClient.handleError)
	socketmodeHandler.Handle(socketmode.EventTypeConnectionError, teamClient.handleConnError)

	return socketmodeHandler.RunEventLoop()
}

func (st *SlackTeamClient) handleConnecting(evt *socketmode.Event, client *socketmode.Client) {
	st.log.Debug("Connecting to Slack...")
}

func (st *SlackTeamClient) handleConnected(evt *socketmode.Event, client *socketmode.Client) {
	st.log.Info("Connected to Slack.")
}

func (st *SlackTeamClient) handleHello(evt *socketmode.Event, client *socketmode.Client) {
	st.log.Debug("Received 'hello' from Slack.")
}

func (st *SlackTeamClient) handleError(evt *socketmode.Event, client *socketmode.Client) {
	ev, ok := evt.Data.(slack.IncomingEventError)
	if !ok {
		st.log.Warn("Invalid event data.", "evt", *evt)
		return
	}
	st.log.Error("Incoming error from Slack.", "err", ev.Error())
}

func (st *SlackTeamClient) handleConnError(evt *socketmode.Event, client *socketmode.Client) {
	ev, ok := evt.Data.(slack.ConnectionErrorEvent)
	if !ok {
		st.log.Warn("Invalid event data.", "evt", *evt)
		return
	}
	st.log.Error("Connection error from Slack.", "err", ev.Error())
}

func (st *SlackTeamClient) handleMessage(evt *socketmode.Event, client *socketmode.Client) {
	eventsAPIEvent, ok := evt.Data.(slackevents.EventsAPIEvent)
	if !ok {
		st.log.Warn("Invalid event data.", "evt", *evt)
		return
	}
	client.Ack(*evt.Request)

	ev, ok := eventsAPIEvent.InnerEvent.Data.(*slackevents.MessageEvent)
	if !ok {
		st.log.Warn("Invalid event data.", "ev", *ev)
		return
	}

	logger := st.log.With("channel", ev.Channel, "ts", ev.TimeStamp)

	if ev.User == st.BotUserID {
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

func (st *SlackTeamClient) handleCommand(evt *socketmode.Event, client *socketmode.Client) {
	ev, ok := evt.Data.(slack.SlashCommand)
	if !ok {
		log.Warn("Invalid event data.", "evt", *evt)
		return
	}
	client.Ack(*evt.Request)

	token := App.webUI.CreateToken(ev.TeamID, ev.ChannelID)
	msg := fmt.Sprintf("Nastavenia tohto kanálu nájdeš tu: %s/%s/%s/%s/", App.config.RootURL, ev.TeamID, ev.ChannelID, token)
	_, err := client.PostEphemeral(ev.ChannelID, ev.UserID, slack.MsgOptionText(msg, false))
	if err != nil {
		var slackErr slack.SlackErrorResponse
		ok := errors.As(err, &slackErr)

		if ok && (slackErr.Err == "channel_not_found" || slackErr.Err == "not_in_channel") {
			st.log.Warn("Received command from a channel I am not in.", "channel", ev.ChannelID, "user", ev.UserID)
			_, _, err := client.PostMessage(ev.UserID, slack.MsgOptionText("⚠️ Predtým, ako môžeš použiť `/buzerator` v nejakom kanáli, musíš ma doňho pridať.", false))
			if err != nil {
				st.log.Error("Could not send command not_in_channel notice.", "channel", ev.ChannelID, "user", ev.UserID)
			}
			return
		}

		st.log.Error("Could not send command reply.", "err", err)
	}
}
