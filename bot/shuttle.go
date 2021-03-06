package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/nlopes/slack"
)

type BusInfo struct {
	MBus    []string `json:"mBus"`
	ABus    []string `json:"aBus"`
	Success bool     `json:"success"`
}

var (
	before_noon []string = []string{"남문", "제2공학관", "과학원", "광복관", "외솔관", "성암관(청송대)", "새천년관", "동문", "경복궁역"}
	after_noon  []string = []string{"남문", "제2공학관", "과학원", "광복관", "외솔관", "성암관(청송대)", "아식설계연구소", "무악학사"}
)

// post error message for shuttle info
func shuttleErrorMessage(bot *BaseBot, channel string) {
	postResponse(bot, channel, ":bus:", "신촌 셔틀버스", "정보를 가져오는데 에러가 발생했습니다.\nhttp://www.yonsei.ac.kr/sc/campus/traffic1.jsp 에서 직접 확인해주세요.")
}

// index to station name
func getStationFromIndex(arr []string, index int) string {
	arr_len := len(arr)
	if index < arr_len {
		return arr[index]
	} else {
		return arr[arr_len-(index-arr_len+1)-1]
	}
}

// get Position(station or transition) from index(retrieved from official site)
func getPositionFromIndex(arr []string, index int) string {
	if index%2 == 0 {
		index /= 2
		return getStationFromIndex(arr, index)
	} else {
		index /= 2
		return fmt.Sprintf("%s -> %s", getStationFromIndex(arr, index), getStationFromIndex(arr, index+1))
	}
}

// make attachement field which contains position & direction of shuttle bus
func makeShuttlePositionAttachmentField(field *slack.AttachmentField, arr []string, index_str string) {
	arr_len := len(arr)
	index, _ := strconv.Atoi(index_str)
	pos := getPositionFromIndex(arr, index)

	field.Title = pos
	var direction string
	// check direction
	if index < (arr_len-1)*2 {
		direction = arr[arr_len-1]
	} else {
		direction = arr[0]
	}
	field.Value = fmt.Sprintf("%s 방향", direction)
	field.Short = true
}

func processShuttleCommand(bot *BaseBot, channel string) {
	// retrieve shuttle bus information from official site
	resp, err := http.Get("http://www.yonsei.ac.kr/_custom/yonsei/_common/shuttle_bus/get_bus_info.jsp")
	if err != nil {
		shuttleErrorMessage(bot, channel)
		return
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	b := buf.Bytes()

	var info BusInfo
	err = json.Unmarshal(b, &info)
	if err != nil {
		shuttleErrorMessage(bot, channel)
		return
	}

	attachments := make([]slack.AttachmentField, len(info.MBus)+len(info.ABus))
	index := 0
	for _, pos := range info.MBus {
		makeShuttlePositionAttachmentField(&attachments[index], before_noon, pos)
		index++
	}
	for _, pos := range info.ABus {
		makeShuttlePositionAttachmentField(&attachments[index], after_noon, pos)
		index++
	}

	attachment := slack.Attachment{
		Color: "#1766ff",
	}
	if len(attachments) == 0 {
		// no info!
		attachment.Text = "현재 운영중인 셔틀버스 정보 없음"
	} else {
		attachment.Text = "셔틀버스 위치"
		attachment.Fields = attachments
	}

	bot.PostMessage(channel, "", slack.PostMessageParameters{
		AsUser:    false,
		IconEmoji: ":bus:",
		Username:  "신촌 셔틀버스",
		Attachments: []slack.Attachment{
			attachment,
		},
	})
}
