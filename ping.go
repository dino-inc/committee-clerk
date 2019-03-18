package main

import (
	"github.com/bwmarrin/discordgo"
	"strconv"
	"strings"
	"time"
)

const (
	DEFAULT_UNANIMOUS  = 10
	AWAIT_UNANIMOUS_ID = "unanimous"
)

var (
	CMD_PING = Command{
		Handler: cmdPing,
		Summary: "Ping the chamber",
		Usage:   "",
	}
	CMD_UNANIMOUS = Command{
		Handler: unanimous,
		Summary: "Record a unanimous agreement",
		Usage:   "[minutes]",
	}

	AWAIT_UNANIMOUS = Await{
		Handler: awaitUnanimous,
		ID:      AWAIT_UNANIMOUS_ID,
		AddErr:  "A unanimous vote is already in-progress",
	}

	OBJECTIONS = []string{
		"i object",
		"objection",
	}
)

func ping(s *discordgo.Session, m *discordgo.MessageCreate, msg string) error {
	role, err := chamberMemberRole(s, m.ChannelID)
	if err != nil {
		return err
	}

	// Set to mentionable
	_, err = s.GuildRoleEdit(m.GuildID, role.ID, role.Name, role.Color,
		role.Hoist, role.Permissions, true)
	if err != nil {
		return err
	}

	// Ping the role
	if msg == "" {
		_, err = s.ChannelMessageSend(m.ChannelID, role.Mention())
	} else {
		_, err = s.ChannelMessageSend(m.ChannelID, msg+" "+role.Mention())
	}

	if err != nil {
		return err
	}

	// Set to unmentionable.
	_, err = s.GuildRoleEdit(m.GuildID, role.ID, role.Name, role.Color,
		role.Hoist, role.Permissions, false)
	return err
}

func cmdPing(s *discordgo.Session, m *discordgo.MessageCreate) error {
	if ok, err := checkAuthorIsSpeaker(s, m); !ok {
		return err
	}

	return ping(s, m, "")
}

func unanimous(s *discordgo.Session, m *discordgo.MessageCreate) error {
	if ok, err := checkAuthorIsSpeaker(s, m); !ok {
		return err
	}

	var duration int
	var err error

	args := strings.Split(m.Content, " ")
	if len(args) > 2 {
		_, err = s.ChannelMessageSend(m.ChannelID, MSG_TOO_MANY_ARGS)
		return err
	} else if len(args) == 2 {
		// arg #1 is a number.
		duration, err = strconv.Atoi(args[1])
		if err != nil {
			_, err = s.ChannelMessageSend(m.ChannelID, MSG_BAD_ARGS)
			return err
		}
	} else {
		duration = DEFAULT_UNANIMOUS
	}

	if ok, err := addAwait(m.ChannelID, s, AWAIT_UNANIMOUS); !ok {
		return err
	}

	err = ping(s, m, "Is there any objection?")

	go func() {
		time.Sleep(time.Duration(duration) * time.Minute)

		removed := removeAwait(m.ChannelID, AWAIT_UNANIMOUS_ID)
		if removed {
			s.ChannelMessageSend(m.ChannelID, "No objection.")
		}
	}()

	return err
}

func awaitUnanimous(s *discordgo.Session, m *discordgo.MessageCreate) error {
	chamber, ok := Chambers[m.ChannelID]
	if !ok {
		// This shouldn't happen; remove our await.
		removeAwait(m.ChannelID, AWAIT_UNANIMOUS_ID)
		return ERR_NOT_A_CHAMBER
	}

	member, err := s.GuildMember(m.GuildID, m.Author.ID)
	if err != nil {
		return err
	}

	if !doesMemberHaveRole(member, chamber.MemberRole) {
		// Ignore if message is from a non-member.
		return nil
	}

	msg := strings.ToLower(m.Content)
	for _, objection := range OBJECTIONS {
		if len(msg) >= len(objection) && msg[:len(objection)] == objection {
			// Member objected; give it the objection.
			_, err = s.ChannelMessageSend(m.ChannelID, "with objection")
			removeAwait(m.ChannelID, AWAIT_UNANIMOUS_ID)
		}
	}

	return err
}
