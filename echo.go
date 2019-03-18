package main

import "github.com/bwmarrin/discordgo"

func startEcho(s *discordgo.Session, m *discordgo.MessageCreate) error {
	_, err := addAwait(m.ChannelID, s, Await{
		handler: awaitEcho,
		id:      "echo",
		adderr:  "An echo session is already in progress",
	})

	return err
}

func endEcho(s *discordgo.Session, m *discordgo.MessageCreate) error {
	var err error
	if ok := removeAwait(m.ChannelID, "echo"); !ok {
		_, err = s.ChannelMessageSend(m.ChannelID, "No echo session is currently in progress")
	}

	return err
}

func awaitEcho(s *discordgo.Session, m *discordgo.MessageCreate) error {
	_, err := s.ChannelMessageSend(m.ChannelID, "The bloke said: "+m.Content)
	return err
}
