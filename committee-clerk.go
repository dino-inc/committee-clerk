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

// Provided by auth.json
type AuthSettings struct {
	Token    string
	ClientID int
}

type Handler func(*discordgo.Session, *discordgo.MessageCreate) error

type Await struct {
	handler Handler
	id      string // Identifies the type of await it is.
	adderr  string // Error to say if an await tries to replace this one
}

type Command struct {
	handler Handler
	summary string
	details string
}

// Configuration
const Prefix = ";"

var Commands = make(map[string]Command)
var Awaits = make(map[string]Await)

func addCommand(name string, cmd Command) {
	Commands[Prefix+name] = cmd
}

// Attempt to attach an await to the channel. Return whether
// successful.
func addAwait(channelID string, s *discordgo.Session, await Await) (bool, error) {
	if prev, exists := Awaits[channelID]; exists {
		// Await already exists for channel; handle appropriately.

		_, err := s.ChannelMessageSend(channelID, prev.adderr)
		return false, err
	}

	Awaits[channelID] = await
	return true, nil
}

// Remove any attached await from the channel if the id
// matches. Return if removed.
func removeAwait(channelID string, id string) bool {
	await, exists := Awaits[channelID]

	if !exists {
		// No await exists.
		return false
	} else if await.id != id {
		// ID doesn't match.
		return false
	} else {
		delete(Awaits, channelID)
		return true
	}
}

func main() {
	// Decode auth.json and store it in the appropriate struct.
	file, err := os.Open("auth.json")
	var auth AuthSettings

	if err != nil {
		log.Fatal(err)
	}

	dec := json.NewDecoder(file)
	if err := dec.Decode(&auth); err != nil {
		log.Fatal(err)
	}

	// Setup the bot.
	dg, err := discordgo.New("Bot " + auth.Token)
	if err != nil {
		log.Fatal(err)
	}

	dg.AddHandler(messageCreate)

	// Add commands
	addCommand("ping", Command{handler: ping})
	addCommand("startecho", Command{handler: startEcho})
	addCommand("endecho", Command{handler: endEcho})

	// Start the bot
	if err = dg.Open(); err != nil {
		log.Fatal("error opening connection,", err)
	}

	// Wait here until an interruption signal is received
	fmt.Println("Committee clerk is now running. Press CTRL-C to exit.")
	fmt.Println("Invite the Committee Clerk with this url:")
	fmt.Println("https://discordapp.com/oauth2/authorize?client_id=" +
		strconv.Itoa(auth.ClientID) + "&permissions=268445776&scope=bot")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close the discord session
	log.Println("Closing Committee Clerk")
	if err = dg.Close(); err != nil {
		log.Fatal("error while closing,", err)
	}
}

// Called every time a new message appears.
func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	// Ignore all messages made any bots.
	if m.Author.Bot {
		return
	}

	cmdstr := strings.Split(m.Content, " ")[0]

	if cmd, ok := Commands[cmdstr]; ok {
		// Redirect data to command handler when appropriate

		if ch, err := s.Channel(m.ChannelID); err != nil {
			log.Println(m.Author, "sent command", m.Content)
		} else {
			// This logically shouldn't happen, but just in case!
			log.Println(m.Author, "from", "#"+ch.Name, "sent command", m.Content)
		}

		if err := cmd.handler(s, m); err != nil {
			log.Println("Error processing command:", err)
		}
	} else if await, ok := Awaits[m.ChannelID]; ok {
		// Redirect data to handler to await if it exists and not a command.

		if ch, err := s.Channel(m.ChannelID); err != nil {
			log.Println(m.Author, "triggered await in #"+ch.Name)
		} else {
			log.Println(m.Author, "triggered await in channel", m.ChannelID)
		}

		if err := await.handler(s, m); err != nil {
			log.Println("Error for await:", err)
		}
	}
}

func ping(s *discordgo.Session, m *discordgo.MessageCreate) error {
	_, err := s.ChannelMessageSend(m.ChannelID, "Pong!")
	return err
}
