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
		Usage:   "<T.C.# | T.C.RES.# | T.C.MOTION.# | T.C.AMDT.# | T.C.CONF.#>",
	}

	AWAIT_ADD_DOCKET_ITEM = Await{
		Handler: awaitAddToDocket,
		ID:      AWAIT_ADD_DOCKET_ITEM_ID,
		AddErr:  "Someone is busy adding an item to the docket.",
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

type PendingDocketItem struct {
	motionClass string
	sponsorName string
	speakerID   string
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
	if ok, err := checkAuthorCanManageChannels(s, m); !ok {
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
		motionClass: args[1],
		sponsorName: sponsor.Username,
		speakerID:   m.Author.ID,
	}

	if ok, err := addAwait(m.ChannelID, s, AWAIT_ADD_DOCKET_ITEM); !ok {
		return err
	}

	_, err = s.ChannelMessageSend(m.ChannelID, "What's the description of the motion?")
	return err
}

func awaitAddToDocket(s *discordgo.Session, m *discordgo.MessageCreate) error {
	docketItem := DocketItems[m.ChannelID]
	if m.Author.ID != docketItem.speakerID {
		// Ignore if the speaker is not giving the bill description.
		return nil
	}

	if ok := removeAwait(m.ChannelID, AWAIT_ADD_DOCKET_ITEM_ID); !ok {
		return nil
	}

	var docket Docket
	if err := apiRequest(s, m, "docket/add", url.Values{
		"motion":  {docketItem.motionClass},
		"sponsor": {docketItem.sponsorName},
		"name":    {m.Content},
	}, &docket); err != nil {
		return err
	}

	_, err := s.ChannelMessageSend(m.ChannelID, "Item added and identified as "+docket.Identifier)
	return err
}

func cmdReadDocketedItem(s *discordgo.Session, m *discordgo.MessageCreate) error {
	if ok, err := checkArgRange(s, m, 1, 1); !ok {
		return err
	}

	args := strings.Split(m.Content, " ")

	var docketItem DocketItem
	if err := apiRequest(s, m, "docket/read", url.Values{
		"identifier": {args[1]},
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
