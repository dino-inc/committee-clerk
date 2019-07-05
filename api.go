package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	AWAIT_ADD_DOCKET_ITEM_ID = "addtodocket"
	AWAIT_DELITEM_ID         = "delitem"
)

var (
	CMD_APIPING = Command{
		Handler: cmdApiPing,
		Summary: "Ping the Website API",
	}
	CMD_ADD_DOCKET_ITEM = Command{
		Handler: cmdAddDocketItem,
		Summary: "Run the command, then provide the description in the next comment. " +
			"Add a new item to the docket.",
		Usage: "<motion|bill|resolution|amendment|confirmation> <@Sponsor>",
	}
	CMD_READ_DOCKETED_ITEM = Command{
		Handler: cmdReadDocketedItem,
		Summary: "Read the docketed item, e.g. T.C.1",
		Usage:   "<MOTION>",
	}
	CMD_COMMENT_DOCKETED_ITEM = Command{
		Handler: cmdCommentDocketedItem,
		Summary: "Set or remove the comment of the docketed item.",
		Usage:   "<MOTION> [COMMENT...]",
	}
	CMD_SET_ITEM_STATUS = Command{
		Handler: cmdSetItemStatus,
		Summary: "Change the status of the docketed item.",
		Usage:   "<MOTION> <STATUS>",
	}
	CMD_PASS = Command{
		Handler: cmdPass,
		Summary: "Pass a docketed item.",
		Usage:   "<MOTION>",
	}
	CMD_FAIL = Command{
		Handler: cmdFail,
		Summary: "Fail a docketed item.",
		Usage:   "<MOTION>",
	}
	CMD_TABLE = Command{
		Handler: cmdTable,
		Summary: "Table a docketed item.",
		Usage:   "<MOTION>",
	}
	CMD_DELITEM = Command{
		Handler: cmdDelitem,
		Summary: "Delete a docketed item.",
		Usage:   "<MOTION>",
	}

	AWAIT_ADD_DOCKET_ITEM = Await{
		Handler: awaitAddToDocket,
		ID:      AWAIT_ADD_DOCKET_ITEM_ID,
		AddErr:  "Someone is busy adding an item to the docket.",
	}
	AWAIT_DELITEM = Await{
		Handler: awaitDelitem,
		ID:      AWAIT_DELITEM_ID,
		AddErr:  "Someone is busy deleting an item from the docket,",
	}
)

type ApiError struct {
	Status int    `json:"status"`
	Error  string `json:"error"`
}

type Ping struct {
	Message string `json:"message"`
}

type Docket struct {
	Identifier string `json:"identifier"`
}

const (
	PENDINGITEM_DESC = iota
	PENDINGITEM_CONF
)

type PendingDocketItem struct {
	motionClass   string
	sponsorName   string
	speakerID     string
	name          string
	pendingStatus int
}

type PendingDeletion struct {
	speakerID  string
	identifier string
}

type DocketItem struct {
	Identifier   string `json:"identifier"`
	MotionStatus string `json:"motionStatus"`
	MotionClass  string `json:"motionClass"`
	ClassNumber  int    `json:"classNumber"`
	Name         string `json:"name"`
	Sponsor      string `json:"sponsor"`
	Comment      string `json:"comment"`
	Date         string `json:"date"`
}

var DocketItems = make(map[string]*PendingDocketItem)
var DocketDeletions = make(map[string]*PendingDeletion)

func readDocketItem(s *discordgo.Session, m *discordgo.MessageCreate, identifier string) error {
	var docketItem DocketItem
	if err := apiRequest(s, m, "docket/read", url.Values{
		"identifier": {identifier},
	}, &docketItem); err != nil {
		return err
	}

	message := fmt.Sprintf(
		"__%s__ *(%s)*\n**Sponsor:** %s\n**Date:** %s\n\n```%s```",
		docketItem.Identifier, docketItem.MotionStatus, docketItem.Sponsor,
		docketItem.Date, docketItem.Name)
	if docketItem.Comment != "" {
		message += fmt.Sprintf("\n**Comment:**\n```%s```", docketItem.Comment)
	}
	_, err := s.ChannelMessageSend(m.ChannelID, message)

	return err
}

func sendApiError(s *discordgo.Session, m *discordgo.MessageCreate, e error) error {
	_, err := s.ChannelMessageSend(m.ChannelID, e.Error()+" <@"+Auth.OwnerID+">")
	return err
}

func apiRequest(s *discordgo.Session, m *discordgo.MessageCreate,
	uri string, params url.Values, dest interface{}) error {

	params.Add("token", Auth.WebToken)

	res, err := http.PostForm(Auth.BaseUri+uri, params)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	tee := io.TeeReader(res.Body, &buf)
	var apiError ApiError

	dec := json.NewDecoder(tee)
	if err := dec.Decode(&apiError); err != nil {
		return err
	}

	if apiError.Status != 200 {
		err := errors.New("API Error " + strconv.Itoa(apiError.Status) + ": " + apiError.Error)
		sendApiError(s, m, err)
		return err
	}

	if dest != nil {
		dec = json.NewDecoder(&buf)
		if err := dec.Decode(dest); err != nil {
			return err
		}
	}

	return nil
}

func cmdApiPing(s *discordgo.Session, m *discordgo.MessageCreate) error {
	var ping Ping
	if err := apiRequest(s, m, "ping", url.Values{}, &ping); err != nil {
		return err
	}

	_, err := s.ChannelMessageSend(m.ChannelID, ping.Message)
	return err
}

func cmdAddDocketItem(s *discordgo.Session, m *discordgo.MessageCreate) error {
	// First check if the user has permissions.
	if ok, err := checkAuthorIsClerk(s, m); !ok {
		return err
	}

	if ok, err := checkArgRange(s, m, 2, 2); !ok {
		return err
	}

	args := strings.Split(m.Content, " ")

	if len(m.Mentions) != 1 {
		_, err := s.ChannelMessageSend(m.ChannelID, MSG_BAD_ARGS)
		return err
	}

	sponsor, err := s.User(m.Mentions[0].ID)
	if err != nil {
		return err
	}

	DocketItems[m.ChannelID] = &PendingDocketItem{
		motionClass:   args[1],
		sponsorName:   sponsor.Username,
		speakerID:     m.Author.ID,
		pendingStatus: PENDINGITEM_DESC,
	}

	if ok, err := addAwait(m.ChannelID, s, AWAIT_ADD_DOCKET_ITEM); !ok {
		return err
	}

	_, err = s.ChannelMessageSend(m.ChannelID, "What's the description of the motion?")
	return err
}

func awaitAddToDocket(s *discordgo.Session, m *discordgo.MessageCreate) error {
	var docketItem *PendingDocketItem = DocketItems[m.ChannelID]
	if m.Author.ID != docketItem.speakerID {
		// Ignore if the speaker is not giving the bill description.
		return nil
	}

	switch docketItem.pendingStatus {
	case PENDINGITEM_DESC:
		docketItem.name = m.Content
		docketItem.pendingStatus = PENDINGITEM_CONF

		_, err := s.ChannelMessageSend(m.ChannelID, "Does this look right to you? (aye/nay)")
		return err
	case PENDINGITEM_CONF:
		vote, err := parseVote(m.Content)

		if err != nil {
			_, err = s.ChannelMessageSend(m.ChannelID, "That response doesn't make sense.")
			return nil
			return err
		} else if vote == Abstained {
			_, err = s.ChannelMessageSend(m.ChannelID, "An absention doesn't make sense here.")
			return nil
		} else if vote == For {
			var docket Docket
			if err := apiRequest(s, m, "docket/add", url.Values{
				"motion":  {docketItem.motionClass},
				"sponsor": {docketItem.sponsorName},
				"name":    {docketItem.name},
			}, &docket); err != nil {
				return err
			}

			if ok := removeAwait(m.ChannelID, AWAIT_ADD_DOCKET_ITEM_ID); !ok {
				return nil
			}

			_, err := s.ChannelMessageSend(m.ChannelID, "Item added and identified as "+docket.Identifier)
			return err
		} else {
			if ok := removeAwait(m.ChannelID, AWAIT_ADD_DOCKET_ITEM_ID); !ok {
				return nil
			}

			_, err := s.ChannelMessageSend(m.ChannelID, "Item ignored.")
			return err

		}
	default:
		_, err := s.ChannelMessageSend(m.ChannelID, "Sorry, something impossible happened. Ping the author, please!")
		return err
	}
}

func cmdReadDocketedItem(s *discordgo.Session, m *discordgo.MessageCreate) error {
	if ok, err := checkArgRange(s, m, 1, 1); !ok {
		return err
	}

	args := strings.Split(m.Content, " ")
	identifier := args[1]

	return readDocketItem(s, m, identifier)
}

func cmdCommentDocketedItem(s *discordgo.Session, m *discordgo.MessageCreate) error {
	// First check if the user has permissions.
	if ok, err := checkAuthorIsClerk(s, m); !ok {
		return err
	}

	if ok, err := checkArgRange(s, m, 1, ARGS_NO_LIMIT); !ok {
		return err
	}

	args := strings.Split(m.Content, " ")
	comment := strings.Join(args[2:], " ")

	if err := apiRequest(s, m, "docket/comment", url.Values{
		"identifier": {args[1]},
		"comment":    {comment},
	}, nil); err != nil {
		return err
	}

	message := "Added comment to " + args[1] + "."
	if len(args) == 2 {
		message = "Removed comment from " + args[1] + "."
	}

	_, err := s.ChannelMessageSend(m.ChannelID, message)
	return err
}

func cmdSetItemStatus(s *discordgo.Session, m *discordgo.MessageCreate) error {
	// First, check if the user has permissions.
	if ok, err := checkAuthorIsClerk(s, m); !ok {
		return err
	}

	if ok, err := checkArgRange(s, m, 2, 2); !ok {
		return err
	}

	args := strings.Split(m.Content, " ")
	identifier := args[1]
	status := args[2]

	if err := apiRequest(s, m, "docket/status", url.Values{
		"identifier": {identifier},
		"status":     {status},
	}, nil); err != nil {
		return err
	}

	message := identifier + " is now considered a(n) " + status + " matter."
	_, err := s.ChannelMessageSend(m.ChannelID, message)
	return err
}

func cmdPass(s *discordgo.Session, m *discordgo.MessageCreate) error {
	if ok, err := checkAuthorIsClerk(s, m); !ok {
		return err
	}

	if ok, err := checkArgRange(s, m, 1, 1); !ok {
		return err
	}

	args := strings.Split(m.Content, " ")
	identifier := args[1]

	if err := apiRequest(s, m, "docket/status", url.Values{
		"identifier": {identifier},
		"status":     {"passed"},
	}, nil); err != nil {
		return err
	}

	message := identifier + " is now considered passed."
	_, err := s.ChannelMessageSend(m.ChannelID, message)
	return err
}

func cmdFail(s *discordgo.Session, m *discordgo.MessageCreate) error {
	if ok, err := checkAuthorIsClerk(s, m); !ok {
		return err
	}

	if ok, err := checkArgRange(s, m, 1, 1); !ok {
		return err
	}

	args := strings.Split(m.Content, " ")
	identifier := args[1]

	if err := apiRequest(s, m, "docket/status", url.Values{
		"identifier": {identifier},
		"status":     {"failed"},
	}, nil); err != nil {
		return err
	}

	message := identifier + " is now considered failed."
	_, err := s.ChannelMessageSend(m.ChannelID, message)
	return err
}

func cmdTable(s *discordgo.Session, m *discordgo.MessageCreate) error {
	if ok, err := checkAuthorIsClerk(s, m); !ok {
		return err
	}

	if ok, err := checkArgRange(s, m, 1, 1); !ok {
		return err
	}

	args := strings.Split(m.Content, " ")
	identifier := args[1]

	if err := apiRequest(s, m, "docket/status", url.Values{
		"identifier": {identifier},
		"status":     {"tabled"},
	}, nil); err != nil {
		return err
	}

	message := identifier + " is now considered tabled."
	_, err := s.ChannelMessageSend(m.ChannelID, message)
	return err
}

func cmdDelitem(s *discordgo.Session, m *discordgo.MessageCreate) error {
	if ok, err := checkAuthorIsClerk(s, m); !ok {
		return err
	}

	if ok, err := checkArgRange(s, m, 1, 1); !ok {
		return err
	}

	args := strings.Split(m.Content, " ")
	identifier := args[1]

	if err := readDocketItem(s, m, identifier); err != nil {
		return err
	}

	if ok, err := addAwait(m.ChannelID, s, AWAIT_DELITEM); !ok {
		return err
	}

	DocketDeletions[m.ChannelID] = &PendingDeletion{
		speakerID:  m.Author.ID,
		identifier: identifier,
	}

	_, err := s.ChannelMessageSend(m.ChannelID, "Are you sure you want to delete this docket item? (aye/nay)")
	return err
}

func awaitDelitem(s *discordgo.Session, m *discordgo.MessageCreate) error {
	deletion := DocketDeletions[m.ChannelID]
	if m.Author.ID != deletion.speakerID {
		// Ignore if the speaker is not confirming the motion deletion.
		return nil
	}

	vote, err := parseVote(m.Content)
	if err != nil {
		_, err := s.ChannelMessageSend(m.ChannelID, "That response doesn't make sense.")
		return err
	} else if vote == Abstained {
		_, err := s.ChannelMessageSend(m.ChannelID, "An absention doesn't make sense here.")
		return err
	} else if vote == For {
		if ok := removeAwait(m.ChannelID, AWAIT_DELITEM_ID); !ok {
			return nil
		}

		if err := apiRequest(s, m, "docket/delitem", url.Values{
			"identifier": {deletion.identifier},
		}, nil); err != nil {
			return err
		}

		_, err := s.ChannelMessageSend(m.ChannelID, "Motion has been deleted.")
		return err
	} else {
		if ok := removeAwait(m.ChannelID, AWAIT_DELITEM_ID); !ok {
			return nil
		}

		_, err := s.ChannelMessageSend(m.ChannelID, "OK, ignoring request to delete item.")
		return err
	}
}
