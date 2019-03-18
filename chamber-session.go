package main

import (
	"github.com/bwmarrin/discordgo"
	"strings"
)

var (
	CMD_CONVENE = Command{
		Handler: cmdConvene,
		Summary: "Start a chamber session.",
	}
	CMD_DISMISS = Command{
		Handler: cmdDismiss,
		Summary: "End the chamber session and schedule an optional later date",
		Usage:   "[time]",
	}
	CMD_ADJOURNSINEDIE = Command{
		Handler: cmdAdjournSineDie,
		Summary: "Adjourn the chamber *sine die*.",
	}
)

func cmdConvene(s *discordgo.Session, m *discordgo.MessageCreate) error {
	_, err := s.ChannelMessageSend(m.ChannelID, "**The chamber is called to order.**")
	return err
}

func cmdDismiss(s *discordgo.Session, m *discordgo.MessageCreate) error {
	args := strings.Split(m.Content, " ")
	var err error

	if len(args) > 1 {
		time := strings.Join(args[1:], " ")
		_, err = s.ChannelMessageSend(m.ChannelID, "**The chamber will reconvene at "+time+".**")
	} else {
		_, err = s.ChannelMessageSend(m.ChannelID, "**The chamber is adjourned.**")
	}

	return err
}

func cmdAdjournSineDie(s *discordgo.Session, m *discordgo.MessageCreate) error {
	_, err := s.ChannelMessageSend(m.ChannelID, "**The chamber is adjourned *sine die*.**")
	return err
}
