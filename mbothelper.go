package mbothelper

import (
	"log"
	"os"
	"os/signal"

	"fmt"
	"github.com/mattermost/platform/model"
)

type BotConfig struct {
	MattermostServer string
	MattermostWSURL  string
	Listen           string
	BotName          string
	UserEmail        string
	UserName         string
	UserPassword     string
	UserLastname     string
	UserFirstname    string
	TeamName         string
	LogChannel       string
	MainChannel      string
	StatusChannel    string
	PluginsDirectory string
	Plugins          []string
	PluginsConfig    map[string]BotConfigPlugin
}

type BotConfigPlugin struct {
	PluginName     string
	Handler        string
	Watcher        string
	MentionHandler string
	HelpHandler    string
	PathPatterns   []string
	PluginConfig   string
	Channels       map[string]*model.Channel
}

var config BotConfig

var client *model.Client4
var webSocketClient *model.WebSocketClient

var BotUser *model.User
var BotTeam *model.Team
var DebuggingChannel *model.Channel
var MainChannel *model.Channel
var StatusChannel *model.Channel

func InitMbotHelper(botConfig BotConfig, client4 *model.Client4) {
	config = botConfig
	client = client4
}

func MakeSureServerIsRunning() {
	if props, resp := client.GetOldClientConfig(""); resp.Error != nil {
		log.Println("There was a problem pinging the Mattermost server.  Are you sure it's running?")
		PrintError(resp.Error)
		os.Exit(1)
	} else {
		log.Println("Server detected and is running version " + props["Version"])
	}
}

func LoginAsTheBotUser() {
	if user, resp := client.Login(config.UserEmail, config.UserPassword); resp.Error != nil {
		log.Println("There was a problem logging into the Mattermost server.  Are you sure ran the setup steps from the README.md?")
		PrintError(resp.Error)
		os.Exit(1)
	} else {
		BotUser = user
	}
}

func UpdateTheBotUserIfNeeded() {
	if BotUser.FirstName != config.UserFirstname || BotUser.LastName != config.UserLastname || BotUser.Username != config.UserName {
		BotUser.FirstName = config.UserFirstname
		BotUser.LastName = config.UserLastname
		BotUser.Username = config.UserName

		if user, resp := client.UpdateUser(BotUser); resp.Error != nil {
			log.Println("We failed to update the Sample Bot user")
			PrintError(resp.Error)
			os.Exit(1)
		} else {
			BotUser = user
			log.Println("Looks like this might be the first run so we've updated the bots account settings")
		}
	}
}

func FindBotTeam() {
	if team, resp := client.GetTeamByName(config.TeamName, ""); resp.Error != nil {
		log.Printf("We failed to get the initial load or we do not appear to be a member of the team '%v'", config.TeamName)
		PrintError(resp.Error)
		os.Exit(1)
	} else {
		BotTeam = team
	}
}

func CreateBotDebuggingChannelIfNeeded() {
	if rchannel, resp := client.GetChannelByName(config.LogChannel, BotTeam.Id, ""); resp.Error != nil {
		log.Println("We failed to get the channels")
		PrintError(resp.Error)
	} else {
		DebuggingChannel = rchannel
		return
	}

	// Looks like we need to create the logging channel
	channel := &model.Channel{}
	channel.Name = config.LogChannel
	channel.DisplayName = "Debugging Channel for bot"
	channel.Purpose = "This is used for logging bot debug messages"
	channel.Type = model.CHANNEL_OPEN
	channel.TeamId = BotTeam.Id
	if rchannel, resp := client.CreateChannel(channel); resp.Error != nil {
		log.Println("We failed to create the channel " + config.LogChannel)
		PrintError(resp.Error)
	} else {
		DebuggingChannel = rchannel
		log.Println("Looks like this might be the first run so we've created the channel " + config.LogChannel)
	}
}

func JoinChannel(channel string, teamId string, userId string) *model.Channel {
	if rchannel, resp := client.GetChannelByName(channel, teamId, ""); resp.Error != nil {
		log.Printf("Failed to get the channel by name: %s", channel)
		PrintError(resp.Error)
	} else {
		if _, resp := client.AddChannelMember(rchannel.Id, userId); resp.Error != nil {
			log.Printf("Failed to join ourselves to the channel: %s", channel)
			PrintError(resp.Error)
		}
		return rchannel
	}
	return nil
}

func SendMsgToChannel(msg string, replyToId string, channelId string) {
	post := &model.Post{}
	post.ChannelId = channelId
	post.Message = msg

	post.RootId = replyToId

	if _, resp := client.CreatePost(post); resp.Error != nil {
		SendMsgToDebuggingChannel(fmt.Sprintf("Failed to send a message to the channel: %s", channelId), "")
	}
}

func SendMsgToDebuggingChannel(msg string, replyToId string) {
	post := &model.Post{}
	post.ChannelId = DebuggingChannel.Id
	post.Message = msg

	post.RootId = replyToId

	if _, resp := client.CreatePost(post); resp.Error != nil {
		log.Println("We failed to send a message to the logging channel")
		PrintError(resp.Error)
	}
}

func PrintError(err *model.AppError) {
	log.Printf("\tError Details:\n\t\t%v\n\t\t%v\n\t\t%v", err.Message, err.Id, err.DetailedError)
}

func SetupGracefulShutdown() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for _ = range c {
			if webSocketClient != nil {
				webSocketClient.Close()
			}

			SendMsgToDebuggingChannel("_"+config.BotName+" has **stopped** running_", "")
			os.Exit(0)
		}
	}()
}
