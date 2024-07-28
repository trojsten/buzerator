package main

import (
	"github.com/charmbracelet/log"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

type SlackTeamClient struct {
	TeamID    string
	BotUserID string
	client    *socketmode.Client
	log       *log.Logger
}

func ConnectTeam(team Team) error {
	api := slack.New(
		team.Token,
		slack.OptionLog(log.Default().WithPrefix("slack api").StandardLog()),
		slack.OptionAppLevelToken(App.config.SlackAppToken),
	)

	_, err := api.AuthTest()
	if err != nil {
		return err
	}
	App.slack[team.ID] = api
	return nil
}
