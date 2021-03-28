package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/hpcloud/tail"
)

type playTimeDayLog struct {
	Day       time.Time   `json:"day"`
	Playtimes []*playTime `json:"playTimes"`
}

type playTime struct {
	PlayerName       string        `json:"playerName"`
	DurationOnServer time.Duration `json:"durationOnServer"`
	LatestStart      time.Time     `json:"latestStart"`
	LatestEnd        time.Time     `json:"latestEnd"`
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

func writeOutDayLog(playTimesMap map[string]*playTime, currentDay time.Time, p string) {
	var pt []*playTime
	for _, v := range playTimesMap {
		fmt.Println(v)
		pt = append(pt, v)
	}

	l := playTimeDayLog{
		Day:       currentDay,
		Playtimes: pt,
	}

	j, _ := json.MarshalIndent(l, "", "")

	filename := fmt.Sprintf(path.Join(p, "playtime_log-%v.json"), currentDay.Format("2006-01-02"))
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

	for logLine := range logTail.Lines {
		fmt.Printf("Got new line: %v\n", logLine)
		t := time.Now()

		line := logLine.Text
		handleLine(line, t, playTimes)

		if current.YearDay() != t.YearDay() {
			fmt.Printf("New day new file: %v\n", current)
			fixEnding(playTimes, current)

			writeOutDayLog(playTimes, current, *timeLogPath)

			playTimes = make(map[string]*playTime)

			current = t
		}
	}
}
