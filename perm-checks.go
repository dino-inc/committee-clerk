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
func doeAuthorHaveRole(s *discordgo.Session, source *discordgo.MessageCreate, testRole string) (bool, error) {
	member, err := s.GuildMember(source.GuildID, source.Author.ID)
	if err != nil {
		return false, err
	}

	return doesMemberHaveRole(member, testRole), nil
}