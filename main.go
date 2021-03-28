package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/hpcloud/tail"
)

type playTimeDayLog struct {
	Day       time.Time           `json:"day"`
	Playtimes []*readablePlayTime `json:"playTimes"`
}

type readablePlayTime struct {
	PlayerName         string        `json:"playerName"`
	LatestStart        time.Time     `json:"latestStart"`
	LatestEnd          time.Time     `json:"latestEnd"`
	DurationOnServer   string        `json:"readableDurationOnServer"`
	DurationOnServerNS time.Duration `json:"durationOnServer"`
}

type playTime struct {
	PlayerName       string
	DurationOnServer time.Duration
	LatestStart      time.Time
	LatestEnd        time.Time
}

func fixEnding(playTimesMap map[string]*playTime, t time.Time) {
	beforeMidnight := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, time.UTC)
	for playerName, playTime := range playTimesMap {
		if playTime.LatestStart.After(playTime.LatestEnd) {
			playTime.LatestEnd = beforeMidnight
			playTime.DurationOnServer += beforeMidnight.Sub(playTime.LatestStart)

			playTimesMap[playerName] = playTime
		}
	}
}

func writeOutDayLog(playTimesMap map[string]*playTime, currentDay time.Time, interrupt bool, p string) {
	var pt []*readablePlayTime
	for _, v := range playTimesMap {
		rpt := &readablePlayTime{
			PlayerName:         v.PlayerName,
			LatestStart:        v.LatestStart,
			LatestEnd:          v.LatestEnd,
			DurationOnServer:   fmt.Sprintf("%s", v.DurationOnServer),
			DurationOnServerNS: v.DurationOnServer,
		}

		pt = append(pt, rpt)
	}

	l := playTimeDayLog{
		Day:       currentDay,
		Playtimes: pt,
	}

	j, _ := json.MarshalIndent(l, "", "")

	var filename string
	if interrupt {
		filename = fmt.Sprintf(path.Join(p, "playtime_log-%v-interrupt.json"), currentDay.Format("2006-01-02T15:04:05"))
	} else {
		filename = fmt.Sprintf(path.Join(p, "playtime_log-%v.json"), currentDay.Format("2006-01-02"))
	}

	ioutil.WriteFile(filename, j, 0644)

}

func getPlayerName(line string) string {
	words := strings.Fields(line)
	if len(words) < 5 {
		return ""
	}

	return words[len(words)-4]
}

func handleLine(line string, t time.Time, playTimes map[string]*playTime) {
	regExpJoined := regexp.MustCompile("joined the game")
	regExpLeft := regexp.MustCompile("left the game")

	//logout
	if regExpLeft.Match([]byte(line)) {

		playerName := getPlayerName(line)
		fmt.Printf("Logout detected: %v\n", playerName)

		current, exists := playTimes[playerName]

		if !exists {
			midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 1, 0, time.UTC)
			current = &playTime{
				PlayerName:  playerName,
				LatestStart: midnight,
			}
		}

		current.LatestEnd = t
		current.DurationOnServer += t.Sub(current.LatestStart)

		playTimes[playerName] = current
	}

	//login
	if regExpJoined.Match([]byte(line)) {
		playerName := getPlayerName(line)
		fmt.Printf("Login detected: %v\n", playerName)

		current, exists := playTimes[playerName]
		if !exists {
			current = &playTime{
				PlayerName: playerName,
			}
		}
		current.LatestStart = t
		playTimes[playerName] = current
	}

}

func main() {
	timeLogPath := flag.String("outputLogPath", "/data/timelog/", "The absolute path to the folder, where the timelog should be written")
	logFilePath := flag.String("minecraftLogPath", "/data/logs/latest.log", "The absolute path to Minecraft's log file")
	flag.Parse()

	logTail, err := tail.TailFile(*logFilePath, tail.Config{Follow: true, ReOpen: true})
	if err != nil {
		os.Exit(1)
	}
	fmt.Printf("Started tailing: %v\n", *logFilePath)

	current := time.Now()
	playTimes := make(map[string]*playTime)

	//write out the current data on sigint/sigterm
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	signal.Notify(c, os.Interrupt, syscall.SIGINT)
	go func() {
		<-c

		fixEnding(playTimes, current)
		writeOutDayLog(playTimes, current, true, *timeLogPath)

		os.Exit(1)
	}()

	//loop over incoming log lines
	for logLine := range logTail.Lines {
		fmt.Printf("Got new line: %v\n", logLine)
		t := time.Now()

		line := logLine.Text
		handleLine(line, t, playTimes)

		if current.YearDay() != t.YearDay() {
			fmt.Printf("New day new file: %v\n", current)
			fixEnding(playTimes, current)

			writeOutDayLog(playTimes, current, false, *timeLogPath)

			playTimes = make(map[string]*playTime)

			current = t
		}
	}
}
