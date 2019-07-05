package main

import (
	"github.com/bwmarrin/discordgo"
	"strings"
)

var CMD_CANNED = Command{
	Handler: canned,
	Summary: "Send a canned response or list all canned messages.",
	Usage:   "[keyword]",
}

func canned(s *discordgo.Session, m *discordgo.MessageCreate) error {
	if ok, err := checkArgRange(s, m, 0, 1); !ok {
		return err
	}

	args := strings.Split(m.Content, " ")

	if len(m.MentionRoles) != 2 {
		response := "*Phrases:*\n"
		for phrase, _ := range Canned {
			response += "\n`" + phrase + "`"
		}

		_, err := s.ChannelMessageSend(m.ChannelID, response)
		return err
	}

	phrase := args[1]
	val, ok := Canned[phrase]
	if ok {
		_, err := s.ChannelMessageSend(m.ChannelID, "**"+val+":**\n\n"+val)
		return err
	} else {
		_, err := s.ChannelMessageSend(m.ChannelID, "'"+phrase+"' isn't a valid phrase")
		return err
	}
}
