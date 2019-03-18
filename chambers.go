package main

import (
	"encoding/json"
	"errors"
	"github.com/bwmarrin/discordgo"
	"log"
	"os"
	"strings"
)

var (
	ERR_NOT_A_CHAMBER = errors.New(MSG_NOT_A_CHAMBER)
)

var (
	CMD_ADD_CHAMBER = Command{
		Handler: addChamber,
		Summary: "Add a chamber to the current channel",
		Usage:   "<member role> <speaker role> [website id]",
	}
	CMD_REMOVE_CHAMBER = Command{
		Handler: removeChamber,
		Summary: "Remove the current channel's chamber",
		Usage:   "",
	}
	CMD_LIST = Command{
		Handler: list,
		Summary: "List all members in the channel's chamber",
		Usage:   "",
	}
)

func isChamber(channelID string) bool {
	_, ok := Chambers[channelID]
	return ok
}

// Return the channel's Chamber Member role.
func chamberMemberRole(s *discordgo.Session, channelID string) (*discordgo.Role, error) {
	chamber, ok := Chambers[channelID]
	if !ok {
		return nil, ERR_NOT_A_CHAMBER
	}

	ch, err := s.State.Channel(channelID)
	if err != nil {
		return nil, err
	}

	return s.State.Role(ch.GuildID, chamber.MemberRole)
}

// Return the channel's Chamber Speaker role.
func chamberSpeakerRole(s *discordgo.Session, channelID string) (*discordgo.Role, error) {
	chamber, ok := Chambers[channelID]
	if !ok {
		return nil, ERR_NOT_A_CHAMBER
	}

	ch, err := s.State.Channel(channelID)
	if err != nil {
		return nil, err
	}

	return s.State.Role(ch.GuildID, chamber.SpeakerRole)
}

// Save the current chambers to the chamber JSON file.
func saveChambers() error {
	file, err := os.Create(CHAMBER_PATH)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(file)
	if err = enc.Encode(Chambers); err != nil {
		return err
	}

	return nil
}

// Set up a chamber for the current channel.
func addChamber(s *discordgo.Session, m *discordgo.MessageCreate) error {
	args := strings.Split(m.Content, " ")

	// Check if they can manage channels first.
	ok, err := canAuthorManageChannels(s, m)
	if err != nil {
		return err
	}
	if !ok {
		_, err := s.ChannelMessageSend(m.ChannelID, "You must Manage Channels to do that!")
		return err
	}

	if len(args) < 3 {
		_, err := s.ChannelMessageSend(m.ChannelID, MSG_TOO_FEW_ARGS)
		return err
	} else if len(args) > 4 {
		_, err := s.ChannelMessageSend(m.ChannelID, MSG_TOO_MANY_ARGS)
		return err
	} else if len(m.MentionRoles) != 2 {
		_, err := s.ChannelMessageSend(m.ChannelID, MSG_BAD_ARGS)
		return err
	}

	member := m.MentionRoles[0]
	speaker := m.MentionRoles[1]
	var apiname string
	if len(args) == 4 {
		apiname = args[3]
	}

	// Add chamber to chambers map.
	Chambers[m.ChannelID] = Chamber{
		MemberRole:  member,
		SpeakerRole: speaker,
		ApiName:     apiname,
	}

	// Update chamber file.
	if err := saveChambers(); err != nil {
		return err
	}
	ch, _ := s.Channel(m.ChannelID)
	log.Println("Added chamber", "#"+ch.Name)

	// React with OK.
	err = s.MessageReactionAdd(m.ChannelID, m.ID, REACT_OK)
	return err
}

// Remove the chamber from the current channel.
func removeChamber(s *discordgo.Session, m *discordgo.MessageCreate) error {
	// Check if they can manage channels first.
	ok, err := canAuthorManageChannels(s, m)
	if err != nil {
		return err
	}
	if !ok {
		_, err := s.ChannelMessageSend(m.ChannelID, "You must Manage Channels to do that!")
		return err
	}

	// Delete chamber and update file.
	delete(Chambers, m.ChannelID)
	if err := saveChambers(); err != nil {
		return err
	}
	ch, _ := s.Channel(m.ChannelID)
	log.Println("Removed chamber", "#"+ch.Name)

	// React with OK.
	err = s.MessageReactionAdd(m.ChannelID, m.ID, REACT_OK)
	return err
}

// Return a slice of all members in the guild that is a Thot Chamber member.
func getChamberMembers(s *discordgo.Session, ch *discordgo.Channel) ([]*discordgo.Member, error) {
	result := make([]*discordgo.Member, 0)

	chamber, ok := Chambers[ch.ID]
	if !ok {
		return nil, ERR_NOT_A_CHAMBER
	}
	memberRole := chamber.MemberRole

	after := ""
	for {
		members, err := s.GuildMembers(ch.GuildID, after, 1000)
		if err != nil {
			return nil, err
		}

		if len(members) == 0 {
			break
		}

		for _, member := range members {
			after = member.User.ID

			if doesMemberHaveRole(member, memberRole) {
				result = append(result, member)
			}
		}
	}

	return result, nil
}

// List all members in the chamber.
func list(s *discordgo.Session, m *discordgo.MessageCreate) error {
	ch, err := s.State.Channel(m.ChannelID)
	if err != nil {
		return err
	}

	members, err := getChamberMembers(s, ch)
	if err != nil {
		if err == ERR_NOT_A_CHAMBER {
			_, err = s.ChannelMessageSend(m.ChannelID, err.Error())
		}
		return err
	}

	message := ch.Mention() + " members:\n\n"
	for _, member := range members {
		user := member.User
		message += user.Username + "#" + user.Discriminator + "\n"
	}

	_, err = s.ChannelMessageSend(m.ChannelID, message)
	return err
}
