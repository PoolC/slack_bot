package bot

import (
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"

	"github.com/nlopes/slack"
)

var (
	testKey     string
	testChannel string
)

type Bot interface {
	getBase() *BaseBot
	onHelloEvent(e *slack.HelloEvent)
	onConnectedEvent(e *slack.ConnectedEvent)
	onMessageEvent(e *slack.MessageEvent)
	onPresenceChangeEvent(e *slack.PresenceChangeEvent)
	onLatencyReportEvent(e *slack.LatencyReport)
	onError(e *slack.RTMError)
	onConnectionError(e *slack.ConnectionErrorEvent)
	onInvalidAuthEvent(e *slack.InvalidAuthEvent)
}

type BaseBot struct {
	*slack.Client
	*slack.RTM
	mention_str string
	stop        *chan struct{}
}

func NewBot(token string, stop *chan struct{}) *BaseBot {
	api := slack.New(token)
	return &BaseBot{api, api.NewRTM(), "", stop}
}

func (bot *BaseBot) MentionStr() string {
	return bot.mention_str
}

func (bot *BaseBot) IsBeginWithMention(e *slack.MessageEvent) bool {
	return strings.HasPrefix(e.Text, bot.MentionStr())
}

func (bot *BaseBot) IsMentioned(e *slack.MessageEvent) bool {
	return strings.Contains(e.Text, bot.MentionStr())
}

func (bot *BaseBot) replySimple(e *slack.MessageEvent, text string) {
	bot.sendSimple(e, fmt.Sprintf("<@%s>: %s", e.User, text))
}

func (bot *BaseBot) sendSimple(e *slack.MessageEvent, text string) {
	bot.SendMessage(bot.NewOutgoingMessage(testKey+text, e.Channel))
}

func (bot *BaseBot) PostMessage(channel string, text string, params slack.PostMessageParameters) (string, string, error) {
	return bot.Client.PostMessage(channel, testKey+text, params)
}

func (bot *BaseBot) getBase() *BaseBot {
	return bot
}

func (bot *BaseBot) onHelloEvent(e *slack.HelloEvent) {
}

func (bot *BaseBot) onConnectedEvent(e *slack.ConnectedEvent) {
	bot_user := bot.GetInfo().User
	bot.mention_str = fmt.Sprintf("<@%s>", bot_user.ID)
	if len(testChannel) == 0 {
		channels, err := bot.GetChannels(true)
		if err != nil {
			log.Fatal(err)
		}
		for _, channel := range channels {
			if channel.Name == "bot_test" {
				testChannel = channel.ID
				log.Print("test channel : " + testChannel)
				break
			}
		}
	}
}

func (bot *BaseBot) onMessageEvent(e *slack.MessageEvent) {
}

func (bot *BaseBot) onPresenceChangeEvent(e *slack.PresenceChangeEvent) {
}

func (bot *BaseBot) onLatencyReportEvent(e *slack.LatencyReport) {
}

func (bot *BaseBot) onError(e *slack.RTMError) {
}

func (bot *BaseBot) onConnectionError(e *slack.ConnectionErrorEvent) {
	log.Println(e.ErrorObj)
}

func (bot *BaseBot) onInvalidAuthEvent(e *slack.InvalidAuthEvent) {
}

func MatchRE(text string, re *regexp.Regexp) ([]string, bool) {
	matched := re.FindStringSubmatch(text)
	return matched, matched != nil
}

func AcceptRE(text string, re *regexp.Regexp) bool {
	_, ok := MatchRE(text, re)
	return ok
}

func StartBot(bot Bot, wg *sync.WaitGroup) {
	bot_base := bot.getBase()
	go bot_base.ManageConnection()

Loop:
	for {
		select {
		case msg := <-bot_base.IncomingEvents:
			switch ev := msg.Data.(type) {
			case *slack.HelloEvent:
				bot.onHelloEvent(ev)
			case *slack.ConnectedEvent:
				bot.onConnectedEvent(ev)
			case *slack.MessageEvent:
				if testKey == "" || testChannel == ev.Channel {
					bot.onMessageEvent(ev)
				}
			case *slack.PresenceChangeEvent:
				bot.onPresenceChangeEvent(ev)
			case *slack.LatencyReport:
				bot.onLatencyReportEvent(ev)
			case *slack.RTMError:
				bot.onError(ev)
			case *slack.ConnectionErrorEvent:
				bot.onConnectionError(ev)
			case *slack.InvalidAuthEvent:
				bot.onInvalidAuthEvent(ev)
				break Loop
			default:
			}
		case _ = <-*bot_base.stop:
			break Loop
		}
	}

	wg.Done()
}

func SetTest(test_key string) {
	testKey = test_key
}
