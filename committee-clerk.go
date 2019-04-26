package main

import (
	"encoding/json"
	"fmt"
	"github.com/bwmarrin/discordgo"
	"log"
	"strconv"
	"strings"

	"os"
	"os/signal"
	"syscall"
)

// Configuration
const (
	PREFIX = ";"

	CHAMBER_PATH = "chambers.json"
	AUTH_PATH    = "auth.json"

	REACT_OK = "\u2705"

	MSG_TOO_MANY_ARGS        = "Too many arguments."
	MSG_TOO_FEW_ARGS         = "Too few arguments."
	MSG_BAD_ARGS             = "Invalid arguments."
	MSG_MUST_MANAGE_CHANNELS = "You need permission to Manage Channels to do that."
	MSG_NOT_A_CHAMBER        = "No chamber is set up for this channel."

	ARGS_NO_LIMIT = -1
)

// Provided by auth.json
type AuthSettings struct {
	Token    string
	ClientID int
	OwnerID  string
	WebToken string
	BaseUri  string
}

type Handler func(*discordgo.Session, *discordgo.MessageCreate) error

type Await struct {
	Handler Handler
	ID      string // Identifies the type of await it is.
	AddErr  string // Error to say if an await tries to replace this one
}

type Command struct {
	Handler Handler
	Summary string
	Usage   string
}

type Chamber struct {
	MemberRole  string `json:"member"`
	SpeakerRole string `json:"speaker"`
	ApiName     string `json:"apiname"`
}

var (
	CMD_HELP = Command{
		Handler: help,
		Summary: "Show a list of all commands available or displays help for a specific command",
		Usage:   "[command name]",
	}
)

// State
var Commands = make(map[string]Command)
var Awaits = make(map[string]Await)
var Chambers map[string]Chamber
var Auth AuthSettings

// Add a command to the bot.
func addCommand(name string, cmd Command) {
	Commands[name] = cmd
}

// Attempt to attach an await to the channel. Return whether
// successful.
func addAwait(channelID string, s *discordgo.Session, await Await) (bool, error) {
	if prev, exists := Awaits[channelID]; exists {
		// Await already exists for channel; handle appropriately.

		_, err := s.ChannelMessageSend(channelID, prev.AddErr)
		return false, err
	}

	Awaits[channelID] = await
	log.Println("Added await '" + await.ID + "'")
	return true, nil
}

// Remove any attached await from the channel if the id
// matches. Return if removed.
func removeAwait(channelID string, id string) bool {
	await, exists := Awaits[channelID]

	if !exists {
		// No await exists.
		return false
	} else if await.ID != id {
		// ID doesn't match.
		return false
	} else {
		delete(Awaits, channelID)
		log.Println("Removed await '" + await.ID + "'")
		return true
	}
}

// Decode the given JSON file and store it in the appropriate data
// structure.
func loadSettings(dest interface{}, src string) error {
	file, err := os.Open(src)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(file)
	if err := dec.Decode(dest); err != nil {
		return err
	}

	return nil
}

// Return whether the arguments are within range, and send an error
// message if it isn't.
func checkArgRange(s *discordgo.Session, m *discordgo.MessageCreate, argMin int, argMax int) (bool, error) {
	args := strings.Split(m.Content, " ")
	if len(args)-1 < argMin {
		_, err := s.ChannelMessageSend(m.ChannelID, MSG_TOO_FEW_ARGS)
		return false, err
	} else if argMax != ARGS_NO_LIMIT && len(args)-1 > argMax {
		_, err := s.ChannelMessageSend(m.ChannelID, MSG_TOO_MANY_ARGS)
		return false, err
	}

	return true, nil
}

func main() {
	// Load the chamber data.
	if err := loadSettings(&Chambers, CHAMBER_PATH); err != nil {
		log.Fatal(err)
	}

	// Load auth settings.
	if err := loadSettings(&Auth, AUTH_PATH); err != nil {
		log.Fatal(err)
	}

	// Setup the bot.
	dg, err := discordgo.New("Bot " + Auth.Token)
	if err != nil {
		log.Fatal(err)
	}

	dg.AddHandler(messageCreate)

	// Add commands
	addCommand("help", CMD_HELP)

	addCommand("addchamber", CMD_ADD_CHAMBER)
	addCommand("removechamber", CMD_REMOVE_CHAMBER)
	addCommand("list", CMD_LIST)
	addCommand("add", CMD_ADD)
	addCommand("remove", CMD_REMOVE)

	addCommand("ping", CMD_PING)
	addCommand("unanimous", CMD_UNANIMOUS)

	addCommand("convene", CMD_CONVENE)
	addCommand("dismiss", CMD_DISMISS)
	addCommand("adjournsinedie", CMD_ADJOURNSINEDIE)

	addCommand("call", CMD_CALL)
	addCommand("endvoting", CMD_ENDVOTING)
	addCommand("resumevoting", CMD_RESUMEVOTING)
	addCommand("cast", CMD_CAST)
	addCommand("getvotes", CMD_GETVOTES)
	addCommand("setvotes", CMD_SETVOTES)

	addCommand("apiping", CMD_APIPING)
	addCommand("addtodocket", CMD_ADD_DOCKET_ITEM)
	addCommand("readitem", CMD_READ_DOCKETED_ITEM)

	// Start the bot
	if err = dg.Open(); err != nil {
		log.Fatal("error opening connection,", err)
	}

	// Wait here until an interruption signal is received
	fmt.Println("Committee clerk is now running. Press CTRL-C to exit.")
	fmt.Println("Invite the Committee Clerk with this url:")
	fmt.Println("https://discordapp.com/oauth2/authorize?client_id=" +
		strconv.Itoa(Auth.ClientID) + "&permissions=268445776&scope=bot")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close the discord session
	log.Println("Closing Committee Clerk")
	if err = dg.Close(); err != nil {
		log.Fatal("error while closing,", err)
	}
}

// Return the command parsed from a string if it exists
func getCommand(content string) (Command, bool) {
	cmdstr := strings.Split(content, " ")[0]
	if !strings.HasPrefix(cmdstr, PREFIX) {
		return Command{}, false
	}

	cmd, ok := Commands[strings.TrimPrefix(cmdstr, PREFIX)]
	if ok {
		return cmd, true
	} else {
		return Command{}, false
	}
}

// Called every time a new message appears.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages made any bots.
	if m.Author.Bot {
		return
	}

	if cmd, ok := getCommand(m.Content); ok {
		// It's a valid command

		if ch, err := s.Channel(m.ChannelID); err == nil {
			log.Println(m.Author, "from", "#"+ch.Name, "sent command", m.Content)
		} else {
			// This logically shouldn't happen, but just in case!
			log.Println(m.Author, "sent command", m.Content)
		}

		// Send data to command handler
		if err := cmd.Handler(s, m); err != nil {
			log.Println("Error processing command:", err)
		}
	} else if await, ok := Awaits[m.ChannelID]; ok {
		// Not a command; redirect message to channel's await if it exists.

		if ch, err := s.Channel(m.ChannelID); err == nil {
			log.Println(m.Author, "triggered await in #"+ch.Name)
		} else {
			log.Println(m.Author, "triggered await in channel", m.ChannelID)
		}

		if err := await.Handler(s, m); err != nil {
			log.Println("Error for await:", err)
		}

		return
	}
}

func help(s *discordgo.Session, m *discordgo.MessageCreate) error {
	args := strings.Split(m.Content, " ")
	var err error

	if ok, err := checkArgRange(s, m, 0, 1); !ok {
		return err
	}

	if len(args)-1 == 1 {
		// Has argument for command.
		cmdname := args[1]
		if cmd, ok := Commands[cmdname]; ok {
			// Command exists.
			_, err = s.ChannelMessageSend(m.ChannelID,
				"**`"+cmdname+"`**: "+cmd.Summary+"\n"+
					"Usage: `"+PREFIX+cmdname+" "+cmd.Usage+"`")
		} else {
			// Command doesn't exist
			_, err = s.ChannelMessageSend(m.ChannelID,
				"Command **`"+cmdname+"`** doesn't exist.")
		}
	} else {
		response := "Commands:"
		// List all commands
		for cmdname, _ := range Commands {
			response += " `" + cmdname + "`"
		}

		_, err = s.ChannelMessageSend(m.ChannelID, response)
	}

	return err
}
