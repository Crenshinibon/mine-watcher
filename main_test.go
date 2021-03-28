package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestMain(t *testing.T) {

}

func TestFixEnding(t *testing.T) {
	type Case struct {
		CaseName        string
		PlayTimesBefore map[string]*playTime
		CurrentTime     time.Time
		PlayTimesAfter  map[string]*playTime
	}

	cases := []Case{
		{
			//missing end ... still playing?
			CaseName: "Missing end",
			PlayTimesBefore: map[string]*playTime{
				"Ralea2": {
					PlayerName:       "Ralea2",
					DurationOnServer: 0,
					LatestStart:      time.Date(2021, 03, 24, 12, 59, 59, 999999999, time.UTC),
					LatestEnd:        time.Time{},
				},
			},
			CurrentTime: time.Date(2021, 03, 24, 12, 35, 0, 0, time.UTC),
			PlayTimesAfter: map[string]*playTime{
				"Ralea2": {
					PlayerName:       "Ralea2",
					DurationOnServer: time.Hour * 11,
					LatestStart:      time.Date(2021, 03, 24, 12, 59, 59, 999999999, time.UTC),
					LatestEnd:        time.Date(2021, 03, 24, 23, 59, 59, 999999999, time.UTC),
				},
			},
		},
		{
			//old end ... still playing?
			CaseName: "Old End",
			PlayTimesBefore: map[string]*playTime{
				"Ralea2": {
					PlayerName:       "Ralea2",
					DurationOnServer: time.Hour * 2,
					LatestStart:      time.Date(2021, 03, 24, 12, 59, 59, 999999999, time.UTC),
					LatestEnd:        time.Date(2021, 03, 24, 11, 35, 0, 0, time.UTC),
				},
			},
			CurrentTime: time.Date(2021, 03, 24, 12, 35, 0, 0, time.UTC),
			PlayTimesAfter: map[string]*playTime{
				"Ralea2": {
					PlayerName:       "Ralea2",
					DurationOnServer: time.Hour*2 + (time.Hour * 11),
					LatestStart:      time.Date(2021, 03, 24, 12, 59, 59, 999999999, time.UTC),
					LatestEnd:        time.Date(2021, 03, 24, 23, 59, 59, 999999999, time.UTC),
				},
			},
		},
	}

	for _, c := range cases {
		fixEnding(c.PlayTimesBefore, c.CurrentTime)

		diff := cmp.Diff(c.PlayTimesAfter, c.PlayTimesBefore)
		if diff != "" {
			fmt.Printf("%s: %s\n", c.CaseName, diff)
			t.Fail()
		}
	}

}

func TestDayLog(t *testing.T) {
	playTimes := map[string]*playTime{
		"Ralea2": {
			PlayerName:       "Ralea2",
			DurationOnServer: time.Minute * 10,
			LatestStart:      time.Date(2021, 03, 24, 12, 35, 0, 0, time.UTC),
			LatestEnd:        time.Date(2021, 03, 24, 12, 45, 0, 0, time.UTC),
		},
	}
	refTime := time.Date(2021, 3, 25, 9, 30, 0, 0, time.UTC)

	p := t.TempDir()
	writeOutDayLog(playTimes, refTime, false, p)
	filePath := path.Join(p, "playtime_log-2021-03-25.json")

	oFile, err := ioutil.ReadFile(filePath)
	if err != nil {
		fmt.Println(filePath)
		fmt.Println(err)
		t.Fail()
	}

	var ptLog playTimeDayLog
	json.Unmarshal(oFile, &ptLog)

	fmt.Println(string(oFile))
	exp := playTimeDayLog{
		Playtimes: []*readablePlayTime{
			{
				PlayerName:         "Ralea2",
				DurationOnServerNS: time.Minute * 10,
				LatestStart:        time.Date(2021, 03, 24, 12, 35, 0, 0, time.UTC),
				LatestEnd:          time.Date(2021, 03, 24, 12, 45, 0, 0, time.UTC),
				DurationOnServer:   "10m0s",
			},
		},
		Day: refTime,
	}

	diff := cmp.Diff(exp, ptLog)
	if diff != "" {
		fmt.Println(diff)
		t.Fail()
	}

}

func TestDayLogInterrupt(t *testing.T) {
	playTimes := map[string]*playTime{
		"Ralea2": {
			PlayerName:       "Ralea2",
			DurationOnServer: time.Minute * 10,
			LatestStart:      time.Date(2021, 03, 24, 12, 35, 0, 0, time.UTC),
			LatestEnd:        time.Date(2021, 03, 24, 12, 45, 0, 0, time.UTC),
		},
	}
	refTime := time.Date(2021, 3, 25, 9, 30, 0, 0, time.UTC)

	p := t.TempDir()
	writeOutDayLog(playTimes, refTime, true, p)
	filePath := path.Join(p, "playtime_log-2021-03-25T09:30:00-interrupt.json")

	_, err := ioutil.ReadFile(filePath)
	if err != nil {
		existing, _ := ioutil.ReadDir(p)
		fmt.Println(existing[0].Name())
		fmt.Println(filePath)
		fmt.Println(err)
		t.Fail()
	}
}

func TestPlayername(t *testing.T) {
	type Case struct {
		Line string
		Name string
	}

	cases := []Case{
		{
			Line: "[13:13:26] [Server thread/INFO]: Ralea2 joined the game",
			Name: "Ralea2",
		},
		{
			Line: "[13:15:52] [Server thread/INFO]: adidfr joined the game",
			Name: "adidfr",
		},
		{
			Line: "[09:54:41] [Server thread/INFO]: Ralea2 left the game",
			Name: "Ralea2",
		},
		{
			Line: "[13:22:28] [Server thread/INFO]: adidfr left the game",
			Name: "adidfr",
		},
		{
			Line: "",
			Name: "",
		},
	}

	for _, c := range cases {
		pName := getPlayerName(c.Line)
		if pName != c.Name {
			t.Fail()
			fmt.Println(pName, "!=", c.Name)
		}
	}

}

func TestHandleLine(t *testing.T) {
	type Case struct {
		Line            string
		PlayTimesBefore map[string]*playTime
		CurrentTime     time.Time
		PlayTimesAfter  map[string]*playTime
	}

	cases := []Case{
		{
			Line:            "[13:13:26] [Server thread/INFO]: Ralea2 joined the game",
			PlayTimesBefore: make(map[string]*playTime),
			CurrentTime:     time.Date(2021, 03, 24, 12, 35, 0, 0, time.UTC),
			PlayTimesAfter: map[string]*playTime{
				"Ralea2": {
					PlayerName:       "Ralea2",
					DurationOnServer: 0,
					LatestStart:      time.Date(2021, 03, 24, 12, 35, 0, 0, time.UTC),
					LatestEnd:        time.Time{},
				},
			},
		},
		{
			Line: "[13:13:26] [Server thread/INFO]: Ralea2 left the game",
			PlayTimesBefore: map[string]*playTime{
				"Ralea2": {
					PlayerName:       "Ralea2",
					DurationOnServer: 0,
					LatestStart:      time.Date(2021, 03, 24, 12, 35, 0, 0, time.UTC),
					LatestEnd:        time.Time{},
				},
			},
			CurrentTime: time.Date(2021, 03, 24, 12, 45, 0, 0, time.UTC),
			PlayTimesAfter: map[string]*playTime{
				"Ralea2": {
					PlayerName:       "Ralea2",
					DurationOnServer: time.Minute * 10,
					LatestStart:      time.Date(2021, 03, 24, 12, 35, 0, 0, time.UTC),
					LatestEnd:        time.Date(2021, 03, 24, 12, 45, 0, 0, time.UTC),
				},
			},
		},
		{
			Line:            "[13:13:26] [Server thread/INFO]: Ralea2 left the game",
			PlayTimesBefore: map[string]*playTime{},
			CurrentTime:     time.Date(2021, 03, 24, 0, 31, 0, 0, time.UTC),
			PlayTimesAfter: map[string]*playTime{
				"Ralea2": {
					PlayerName:       "Ralea2",
					DurationOnServer: time.Minute * 31,
					LatestStart:      time.Date(2021, 03, 24, 0, 0, 0, 0, time.UTC),
					LatestEnd:        time.Date(2021, 03, 24, 0, 31, 0, 0, time.UTC),
				},
			},
		},
	}

	for _, c := range cases {

		handleLine(c.Line, c.CurrentTime, c.PlayTimesBefore)
		diff := cmp.Diff(c.PlayTimesBefore, c.PlayTimesAfter)
		if diff != "" {
			fmt.Println(diff)
			t.Fail()
		}
	}
}
