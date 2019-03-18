package main

import "github.com/bwmarrin/discordgo"

// Return true if the author can manage channels.
func checkAuthorCanManageChannels(s *discordgo.Session, source *discordgo.MessageCreate) (bool, error) {
	authorID := source.Author.ID
	channelID := source.ChannelID

	perms, err := s.State.UserChannelPermissions(authorID, channelID)
	if err != nil {
		return false, err
	}

	ok := perms&discordgo.PermissionManageChannels == discordgo.PermissionManageChannels
	if !ok {
		_, err = s.ChannelMessageSend(source.ChannelID, MSG_MUST_MANAGE_CHANNELS)
	}

	return ok, err
}

// Return true if the member has the specified role.
func doesMemberHaveRole(member *discordgo.Member, testRole string) bool {
	for _, role := range member.Roles {
		if role == testRole {
			return true
		}
	}

	return false
}

// Return true if the author has the specified role.
func checkAuthorHasRole(s *discordgo.Session, source *discordgo.MessageCreate, testRole string) (bool, error) {
	member, err := s.GuildMember(source.GuildID, source.Author.ID)
	if err != nil {
		return false, err
	}

	ok := doesMemberHaveRole(member, testRole)
	if !ok {
		// Print error Message
		var role *discordgo.Role
		role, err = s.State.Role(source.GuildID, testRole)
		if err != nil {
			return ok, err
		}

		_, err = s.ChannelMessageSend(source.ChannelID, "You must be a "+role.Name)
	}

	return ok, err
}

// Return true if the author is a Speaker
func checkAuthorIsSpeaker(s *discordgo.Session, m *discordgo.MessageCreate) (bool, error) {
	chamber, ok := Chambers[m.ChannelID]
	if !ok {
		_, err := s.ChannelMessageSend(m.ChannelID, MSG_NOT_A_CHAMBER)
		return false, err
	}

	// Check if sender is a chamber speaker
	return checkAuthorHasRole(s, m, chamber.SpeakerRole)
}
