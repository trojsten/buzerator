package main

import (
	"fmt"

	"github.com/charmbracelet/log"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/socketmode"
)

func ConnectSlack() {
	App.slack = make(map[string]*socketmode.Client)

	teams, err := ListTeams()
	if err != nil {
		log.Error("Could not list teams.", "err", err)
		return
	}

	for _, team := range teams {
		go team.Connect()
	}
}

func ListChannelMembers(teamID string, channel string) ([]string, error) {
	client, ok := App.slack[teamID]
	if !ok {
		return []string{}, fmt.Errorf("not connected to team %s", teamID)
	}

	var allUsers []string
	cursor := "_initial"

	for cursor != "" {
		var response []string
		var err error

		if cursor == "_initial" {
			cursor = ""
		}

		response, cursor, err = client.GetUsersInConversation(&slack.GetUsersInConversationParameters{
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

func LoadMemberName(teamID string, user string) (string, error) {
	client, ok := App.slack[teamID]
	if !ok {
		return "", fmt.Errorf("not connected to team %s", teamID)
	}

	resp, err := client.GetUserInfo(user)
	if err != nil {
		return "", err
	}
	return resp.RealName, err
}
