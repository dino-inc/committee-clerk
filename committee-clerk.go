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

type Command func(*discordgo.Session, *discordgo.MessageCreate) error

// Configuration
const Prefix = ";"

var Commands map[string]Command = make(map[string]Command)

func addCommand(name string, cmd Command) {
	Commands[Prefix+name] = cmd
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
	addCommand("ping", ping)

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
		ch, err := s.Channel(m.ChannelID)

		if err != nil {
			log.Println(m.Author, "sent command", m.Content)
		} else {
			// This logically shouldn't happen, but just in case!
			log.Println(m.Author, "from", "#"+ch.Name, "sent command", m.Content)
		}

		if err := cmd(s, m); err != nil {
			log.Println("Error:", err)
		}
	}
}

func ping(s *discordgo.Session, m *discordgo.MessageCreate) error {
	_, err := s.ChannelMessageSend(m.ChannelID, "Pong!")
	return err
}
