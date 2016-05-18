package bot

import (
	"fmt"
	"log"
	"regexp"

	"strings"

	"encoding/json"

	"time"

	"strconv"

	. "github.com/PoolC/slack_bot/util"
	"github.com/marcmak/calc/calc"
	"github.com/nlopes/slack"
	"github.com/robfig/cron"
)

type Meu struct {
	*BaseBot
	rc        RedisClient
	timetable map[string]*TimeTable
	cr        *cron.Cron
}

var (
	calc_re        *regexp.Regexp = regexp.MustCompile("^계산하라 메우 (.+)")
	et_register    *regexp.Regexp = regexp.MustCompile("에타 등록 ([^ ]+)")
	party_register *regexp.Regexp = regexp.MustCompile("(?:(\\d+)월 *(\\d+)일 *)?(\\d+)시(?: *(\\d+)분)?(.+)")
)

func NewMeu(token string, stop *chan struct{}, redisClient RedisClient) *Meu {
	c := cron.New()
	c.Start()
	return &Meu{NewBot(token, stop), redisClient, map[string]*TimeTable{}, c}
}

func (bot *Meu) onConnectedEvent(e *slack.ConnectedEvent) {
	bot.BaseBot.onConnectedEvent(e)
	rescheduleParty(bot)
}

func (bot *Meu) onMessageEvent(e *slack.MessageEvent) {
	message := meuMessageProcess(bot, e)
	switch message.(type) {
	case string:
		bot.sendSimple(e, message.(string))
	}
}

func meuMessageProcess(bot *Meu, e *slack.MessageEvent) interface{} {
	switch {
	case e.Text == "메우, 멱살":
		return "사람은 일을 하고 살아야한다. 메우"
	case e.Text == "메우메우 펫탄탄":
		return `메메메 메메메메 메우메우
메메메 메우메우
펫땅펫땅펫땅펫땅 다이스키`
	}

	text := strings.TrimSpace(e.Text)
	if matched, ok := MatchRE(text, calc_re); ok {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Recovered : %g\n", r)
				bot.replySimple(e, "에러났다 메우")
			}
		}()
		return fmt.Sprintf("%f", calc.Solve(matched[1]))
	} else {
		specialResponses(bot.getBase(), e)
	}
	if bot.IsBeginWithMention(e) {
		if matched, ok := MatchRE(text, et_register); ok {
			// 에브리타임 기록
			et_nick := matched[1]
			bot.rc.Set(fmt.Sprintf("et_nick_%s", e.User), et_nick, 0)
			bot.replySimple(e, "기억했다 메우. 시간표를 가져오겠다 메우.\n시간이 좀 걸릴거다 메우.")

			user, _ := bot.GetUserInfo(e.User)
			go getEveryTimeTable(bot, e.User, user.Name, et_nick)
		} else if strings.HasSuffix(text, "다음 시간") {
			// 에브리타임 다음 시간
			log.Printf("%q", bot.timetable)
			timetable, exists := bot.timetable[e.User]
			if !exists {
				log.Print("Get from redis")
				result := bot.rc.Get(TimeTableKeyName(e.User))
				if result == nil {
					bot.replySimple(e, "시간표 정보가 없다 메우. 등록부터 해달라 메우.")
					return nil
				}

				bytes, _ := result.Bytes()
				timetable = &TimeTable{}
				if json.Unmarshal(bytes, timetable) != nil {
					bot.replySimple(e, "저장된 시간표가 이상하다 메우. 새로 등록해달라 메우.")
					return nil
				}

				bot.timetable[e.User] = timetable
			}

			now := time.Now()
			weekDay := int(now.Weekday() - 1)
			curHour := now.Hour()*12 + now.Minute()/5
			var nextEvent *TimeTableEvent
			if weekDay < 0 || weekDay >= 5 {
				nextEvent = nil
			} else {
				log.Printf("%q", timetable)
				todayEvents := timetable.Days[weekDay].Events
				for _, event := range todayEvents {
					if event.StartTime > curHour {
						nextEvent = &event
						break
					}
				}
			}
			if nextEvent == nil {
				bot.replySimple(e, "오늘은 더 이상 수업이 없다 메우.")
			} else {
				subject := &timetable.Subjects[nextEvent.Id]

				bot.PostMessage(e.Channel, fmt.Sprintf("다음 수업은 \"%s\"다 메우.", subject.Name), slack.PostMessageParameters{
					AsUser:    false,
					IconEmoji: ":meu:",
					Username:  "시간표 알려주는 메우",
					Attachments: []slack.Attachment{
						nextEvent.toSlackAttachment(subject),
					},
				})
			}
		} else if matched, ok := MatchRE(text, party_register); ok {
			now := time.Now()
			month, err := strconv.Atoi(matched[1])
			if err != nil {
				month = int(now.Month())
			}
			day, err := strconv.Atoi(matched[2])
			if err != nil {
				day = now.Day()
			}
			hour, err := strconv.Atoi(matched[3])
			min, err := strconv.Atoi(matched[4])
			if err != nil {
				min = 0
			}
			keyword := matched[5]

			date := time.Date(now.Year(), time.Month(month), day, hour, min, 0, 0, now.Location())
			if date.Before(now) {
				bot.replySimple(e, "과거에 대해서 파티를 모집할 수 없다 메우")
				return nil
			}
			key := partyKey(&date, keyword)
			cardinal := bot.rc.SetAdd(key, e.User)
			bot.replySimple(e, "파티 대기에 들어갔다 메우.")
			if cardinal.Val() == 1 {
				scheduleParty(bot, &date, keyword)
			}
		}
	}

	return nil
}
