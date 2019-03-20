package main

import (
	"fmt"
	"github.com/bwmarrin/discordgo"
	"math"
	"strconv"
	"strings"
	"time"
)

const (
	For Vote = iota
	Against
	Abstained
)

const (
	AWAIT_CALL_ID      = "call"
	MSG_NO_CALL        = "There is no active roll call vote"
	MSG_NO_RECENT_CALL = "There hasn't been any recent roll call vote"

	PassNumDefault = 1
	PassDenDefault = 2
)

var (
	CMD_CALL = Command{
		Handler: cmdCall,
		Summary: "Start a roll-call vote for the chamber",
		Usage:   "[minutes] [<ayes> <total>]",
	}
	CMD_ENDVOTING = Command{
		Handler: cmdEndVoting,
		Summary: "Stop the chamber's roll-call vote early",
	}
	CMD_CAST = Command{
		Handler: cmdCast,
		Summary: "Cast a vote for someone else",
		Usage:   "<member> <aye|nay|present>",
	}
	CMD_GETVOTES = Command{
		Handler: cmdGetVotes,
		Summary: "List the votes of the current active or last active roll call",
	}
	CMD_SETVOTES = Command{
		Handler: cmdSetVotes,
		Summary: "Set the votes required to agree to a motion",
	}

	AWAIT_CALL = Await{
		Handler: awaitCall,
		ID:      AWAIT_CALL_ID,
		AddErr:  "A roll-call vote is already in progress",
	}

	VOTE_REACTS = map[Vote]string{
		For:       "\u2705",
		Against:   "\u274c",
		Abstained: "\u2796",
	}

	VOTE_FOR = []string{
		"yea",
		"aye",
		"jeff",
		"yeet",
		"non't",
		"nont",
	}
	VOTE_AGAINST = []string{
		"nay",
		"nae",
		"gay",
		"yesn't",
		"yesnt",
	}
	VOTE_ABSTAINED = []string{
		"abstain",
		"present",
	}
)

type Vote int

var RollCalls = make(map[string]*RollCall)

func (v Vote) String() string {
	switch v {
	case For:
		return "For"
	case Against:
		return "Against"
	case Abstained:
		return "Abstained"
	default:
		return "???"
	}
}

type RollCall struct {
	votes    map[string]Vote // Map from UserID to vote
	members  []string        // List of UserID's of chamber members since the start of the vote
	quorum   int             // Pre-calculated minimum number of votes to call quorum
	timedOut bool
	passNum  int
	passDen  int
	active   bool
}

// Return whether a roll call vote is active in the given channel.
func isActiveRollCall(channelID string) bool {
	rollCall, ok := RollCalls[channelID]
	if !ok {
		return false
	} else {
		return rollCall.active
	}
}

// Return whethera memberID matches a voting member in the roll call vote.
func (r RollCall) isMember(memberID string) bool {
	for _, member := range r.members {
		if memberID == member {
			return true
		}
	}

	return false
}

// Return a string fraction of the voting requirements
func (r RollCall) PassReqtoa() string {
	return strconv.Itoa(r.passNum) + "/" + strconv.Itoa(r.passDen)
}

// Return whether the votes meet quorum.
func (r RollCall) QuorumMet() bool {
	return len(r.votes) >= r.quorum
}

// Interprets a string content and gives the corresponding vote. If s
// contains an invalid phrase, it instead returns strconv.ErrSyntax
// and -1 as a vote.
func parseVote(content string) (Vote, error) {
	content = strings.ToLower(content)

	for _, aye := range VOTE_FOR {
		if strings.HasPrefix(content, aye) {
			return For, nil
		}
	}

	for _, nay := range VOTE_AGAINST {
		if strings.HasPrefix(content, nay) {
			return Against, nil
		}
	}

	for _, present := range VOTE_ABSTAINED {
		if strings.HasPrefix(content, present) {
			return Abstained, nil
		}
	}

	return -1, strconv.ErrSyntax
}

func (r *RollCall) countVotes() (ayes int, nays int, absents int) {
	for _, vote := range r.votes {
		switch vote {
		case For:
			ayes++
		case Against:
			nays++
		case Abstained:
			absents++
		}
	}

	return ayes, nays, absents
}

// Stop the roll call vote, remove its associated await, and return
// whether successful and any corresponding errors.
func stopRollCall(s *discordgo.Session, channelID string) (bool, error) {
	if ok := removeAwait(channelID, AWAIT_CALL_ID); !ok {
		return false, nil
	}

	var (
		rollCall     = RollCalls[channelID]
		motionPassed = true
		total        = len(rollCall.votes)
		passReq      = float64(rollCall.passNum) / float64(rollCall.passDen)
	)

	rollCall.active = false
	ayes, nays, absents := rollCall.countVotes()

	if len(rollCall.votes) == 0 {
		motionPassed = false
	} else if float64(ayes)/float64(total) <= passReq {
		motionPassed = false
	}

	reply := "The Yeas and Nays are " +
		strconv.Itoa(ayes) + " - " + strconv.Itoa(nays)
	if absents > 0 {
		reply += " with " + strconv.Itoa(absents) + " absentions"
	}
	reply += ". with " + rollCall.PassReqtoa() + " "

	if motionPassed {
		reply += "voting in the affirmative, the motion is agreed to."
	} else {
		reply += "not able to vote in the affirmative, the motion is not agreed to."
	}

	_, err := s.ChannelMessageSend(channelID, reply)
	return true, err
}

func cmdCall(s *discordgo.Session, m *discordgo.MessageCreate) error {
	var (
		args     = strings.Split(m.Content, " ")
		passNum  = PassNumDefault
		passDen  = PassDenDefault
		duration = -1
	)

	if ok, err := checkAuthorIsSpeaker(s, m); !ok {
		return err
	}

	if len(args) > 4 {
		_, err := s.ChannelMessageSend(m.ChannelID, MSG_TOO_MANY_ARGS)
		return err
	} else if len(args) == 4 {
		// ;call <len> <num> <den>
		var err error

		duration, err = strconv.Atoi(args[1])
		if err != nil {
			_, err = s.ChannelMessageSend(m.ChannelID, MSG_BAD_ARGS)
			return err
		}

		passNum, err = strconv.Atoi(args[2])
		if err != nil {
			_, err = s.ChannelMessageSend(m.ChannelID, MSG_BAD_ARGS)
			return err
		}

		passDen, err = strconv.Atoi(args[3])
		if err != nil {
			_, err = s.ChannelMessageSend(m.ChannelID, MSG_BAD_ARGS)
			return err
		}
	} else if len(args) == 3 {
		// ;call <num> <den>
		var err error

		passNum, err = strconv.Atoi(args[1])
		if err != nil {
			_, err = s.ChannelMessageSend(m.ChannelID, MSG_BAD_ARGS)
			return err
		}

		passDen, err = strconv.Atoi(args[2])
		if err != nil {
			_, err = s.ChannelMessageSend(m.ChannelID, MSG_BAD_ARGS)
			return err
		}
	} else if len(args) == 2 {
		// ;call <len>
		var err error

		duration, err = strconv.Atoi(args[1])
		if err != nil {
			_, err = s.ChannelMessageSend(m.ChannelID, MSG_BAD_ARGS)
			return err
		}
	}

	if ok, err := addAwait(m.ChannelID, s, AWAIT_CALL); !ok {
		return err
	}

	channel, err := s.State.Channel(m.ChannelID)
	if err != nil {
		return err
	}

	var members []*discordgo.Member
	members, err = getChamberMembers(s, channel)
	if err != nil {
		return err
	}
	memberIDs := make([]string, len(members))

	for i, member := range members {
		memberIDs[i] = member.User.ID
	}

	// Store roll call data.
	rollCall := RollCall{
		votes:    make(map[string]Vote),
		members:  memberIDs,
		quorum:   int(math.Floor(float64(len(members))/2.0)) + 1,
		timedOut: false,
		passNum:  passNum,
		passDen:  passDen,
		active:   true,
	}
	RollCalls[m.ChannelID] = &rollCall

	// Start populating the roll call reply.
	content := ""
	for _, member := range members {
		content += "Mr. " + member.Mention() + "\n"
	}

	content += "\nYou have "
	if duration > 0 {
		content += strconv.Itoa(duration) + " minute"
		if duration > 1 {
			content += "s"
		}
	} else {
		content += "unlimited time"
	}

	content += " with " + rollCall.PassReqtoa() +
		" required to vote in the affirmative. The "
	if duration > 0 {
		content += "clock is on."
	} else {
		content += "vote is on."
	}

	if duration > 0 {
		go func() {
			time.Sleep(time.Duration(duration) * time.Minute)
			rollCall.timedOut = true

			if rollCall.QuorumMet() {
				stopRollCall(s, m.ChannelID)
			} else {
				response := "***Quorum is " + strconv.Itoa(rollCall.quorum) +
					". There are currently " + strconv.Itoa(len(rollCall.votes)) +
					" votes.***\n*Is there anyone who would like to cast or change a vote?*"
				s.ChannelMessageSend(m.ChannelID, response)
			}
		}()
	}

	_, err = s.ChannelMessageSend(m.ChannelID, content)
	return err
}

func awaitCall(s *discordgo.Session, m *discordgo.MessageCreate) error {
	rollCall := RollCalls[m.ChannelID]
	var err error

	// Add a vote to the roster if they're a member.
	if rollCall.isMember(m.Author.ID) {
		vote, err := parseVote(m.Content)
		if err == nil {
			rollCall.votes[m.Author.ID] = vote

			err = s.MessageReactionAdd(m.ChannelID, m.ID, VOTE_REACTS[vote])
			if err != nil {
				return err
			}
		}
	}

	if rollCall.timedOut && rollCall.QuorumMet() {
		_, err = stopRollCall(s, m.ChannelID)
	}

	return err
}

func cmdCast(s *discordgo.Session, m *discordgo.MessageCreate) error {
	if ok, err := checkAuthorIsSpeaker(s, m); !ok {
		return err
	}

	args := strings.Split(m.Content, " ")
	if len(args) > 3 {
		_, err := s.ChannelMessageSend(m.ChannelID, MSG_TOO_MANY_ARGS)
		return err
	} else if len(args) < 3 {
		_, err := s.ChannelMessageSend(m.ChannelID, MSG_TOO_FEW_ARGS)
		return err
	} else if len(m.Mentions) != 1 {
		_, err := s.ChannelMessageSend(m.ChannelID, MSG_BAD_ARGS)
		return err
	}

	voteString := args[2]
	vote, err := parseVote(voteString)
	if err != nil {
		_, err := s.ChannelMessageSend(m.ChannelID, "'"+voteString+"' is not a valid vote.")
		return err
	}

	if !isActiveRollCall(m.ChannelID) {
		_, err := s.ChannelMessageSend(m.ChannelID, MSG_NO_CALL)
		return err
	}

	rollCall := RollCalls[m.ChannelID]
	castee := m.Mentions[0]

	if rollCall.isMember(castee.ID) {
		rollCall.votes[castee.ID] = vote
		_, err := s.ChannelMessageSend(m.ChannelID, "Recorded '"+vote.String()+
			"' for "+castee.Username+".")
		return err
	} else {
		_, err := s.ChannelMessageSend(m.ChannelID, castee.Username+" is not a voting member.")
		return err
	}
}

func cmdSetVotes(s *discordgo.Session, m *discordgo.MessageCreate) error {
	if ok, err := checkAuthorIsSpeaker(s, m); !ok {
		return err
	}

	if ok, err := checkArgRange(s, m, 2, 2); !ok {
		return err
	}

	var num, den int
	var err error
	args := strings.Split(m.Content, " ")

	num, err = strconv.Atoi(args[1])
	if err != nil {
		_, err = s.ChannelMessageSend(m.ChannelID, MSG_BAD_ARGS)
		return err
	}

	den, err = strconv.Atoi(args[2])
	if err != nil {
		_, err = s.ChannelMessageSend(m.ChannelID, MSG_BAD_ARGS)
		return err
	}

	rollCall := RollCalls[m.ChannelID]
	rollCall.passNum = num
	rollCall.passDen = den

	_, err = s.ChannelMessageSend(m.ChannelID,
		rollCall.PassReqtoa()+" will require to vote in the affirmative to pass the motion.")
	return err
}

func cmdGetVotes(s *discordgo.Session, m *discordgo.MessageCreate) error {
	rollCall, ok := RollCalls[m.ChannelID]
	if !ok {
		_, err := s.ChannelMessageSend(m.ChannelID, MSG_NO_RECENT_CALL)
		return err
	}

	ayes, nays, absents := rollCall.countVotes()
	content := "*The Yeas and Nays "
	if rollCall.active {
		content += "are currently "
	} else {
		content += "were "
	}
	content += fmt.Sprintf("%d - %d with %d absentions:*\n\n", ayes, nays, absents)

	for userID, vote := range rollCall.votes {
		user, err := s.User(userID)
		if err != nil {
			return err
		}

		content += user.Username + ": " + vote.String() + "\n"
	}

	_, err := s.ChannelMessageSend(m.ChannelID, content)
	return err
}

func cmdEndVoting(s *discordgo.Session, m *discordgo.MessageCreate) error {
	if ok, err := checkAuthorIsSpeaker(s, m); !ok {
		return err
	}

	ok, err := stopRollCall(s, m.ChannelID)
	if err != nil {
		return err
	}

	if !ok {
		_, err = s.ChannelMessageSend(m.ChannelID, MSG_NO_CALL)
	}

	return err
}
