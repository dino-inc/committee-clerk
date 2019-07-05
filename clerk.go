package main

import (
	"encoding/json"
	"github.com/bwmarrin/discordgo"
	"os"
)

var (
	CMD_ADDCLERK = Command{
		Handler: addClerk,
		Summary: "Add a user to the approved clerk list.",
		Usage:   "<member> ...",
	}
	CMD_REMOVECLERK = Command{
		Handler: removeClerk,
		Summary: "Remove a user from the approved clerk list.",
		Usage:   "<member> ...",
	}
)

func saveClerks() error {
	file, err := os.Create(CLERK_PATH)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(file)
	enc.Encode(Clerks)

	return file.Close()
}

func addClerk(s *discordgo.Session, m *discordgo.MessageCreate) error {
	// First check if the user has permissions.
	if ok, err := checkAuthorCanManageChannels(s, m); !ok {
		return err
	}

	if ok, err := checkArgRange(s, m, 1, ARGS_NO_LIMIT); !ok {
		return err
	}

	response := ""
	for _, user := range m.Mentions {
		alreadyClerk := false
		for _, v := range Clerks {
			if user.ID == v {
				alreadyClerk = true
				response += user.Username + " is already a clerk.\n"
				break
			}
		}

		if !alreadyClerk {
			Clerks = append(Clerks, user.ID)
			response += "Added " + user.Username + " as a clerk.\n"
		}
	}

	if err := saveClerks(); err != nil {
		return err
	}

	_, err := s.ChannelMessageSend(m.ChannelID, response)
	return err
}

func removeClerk(s *discordgo.Session, m *discordgo.MessageCreate) error {
	// First check if user has permissions.
	if ok, err := checkAuthorCanManageChannels(s, m); !ok {
		return err
	}

	if ok, err := checkArgRange(s, m, 1, ARGS_NO_LIMIT); !ok {
		return err
	}

	response := ""
	for _, user := range m.Mentions {
		for i := 0; i < len(Clerks); i++ {
			if Clerks[i] == user.ID {
				response += "Removed " + user.Username + " from clerkhood.\n"
				Clerks = append(Clerks[:i], Clerks[i+1:]...)

				i--
			}
		}
	}

	if err := saveClerks(); err != nil {
		return err
	}

	_, err := s.ChannelMessageSend(m.ChannelID, response)
	return err
}
