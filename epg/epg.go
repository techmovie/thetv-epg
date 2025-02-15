package epg

import (
	"encoding/xml"
	"fmt"
	"os"
	"strconv"
	"sync"
	"thetv-apg/consts"
	"thetv-apg/tv"
	"time"

	"gopkg.in/yaml.v3"
)

type TheTVEPG struct {
	XMLName    xml.Name        `xml:"tv"`
	Channels   []*EPGChannel   `xml:"channel"`
	Programmes []*EPGProgramme `xml:"programme"`
}
type EPGChannel struct {
	XMLName     xml.Name `xml:"channel"`
	ID          string   `xml:"id,attr"`
	DisplayName string   `xml:"display-name"`
}

type EPGProgramme struct {
	XMLName xml.Name `xml:"programme"`
	Channel string   `xml:"channel,attr"`
	Start   string   `xml:"start,attr"`
	Stop    string   `xml:"stop,attr"`
	Title   string   `xml:"title"`
	Desc    string   `xml:"desc"`
}

func GenerateEPGForTVList() error {
	channelList, err := LoadYaml()
	if err != nil {
		return err
	}
	res := &TheTVEPG{}
	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	for _, channel := range channelList {
		wg.Add(1)
		go func(channel *tv.TheTV) {
			defer wg.Done()
			path := channel.Path
			if channel.PathAlias != "" {
				path = channel.PathAlias
			}
			tvSchedule, err := tv.GetTVSchedule(channel.ID, path)
			if err != nil {
				fmt.Printf("Warning: failed to fetch schedule for channel %s (%s): %v\n", channel.Name, channel.ID, err)
				return
			}
			mu.Lock()
			res.Channels = append(res.Channels, &EPGChannel{
				ID:          channel.ID,
				DisplayName: channel.Name,
			})
			mu.Unlock()
			for _, schedule := range tvSchedule {
				startTime, err := time.Parse("2006-01-02T15:04:05+00:00", schedule.DataListDatetime)
				if err != nil {
					fmt.Println("Error parsing time:", err)
					continue
				}
				duration, _ := strconv.Atoi(schedule.DataDuration)
				endTime := startTime.Add(time.Duration(duration) * time.Minute)
				title := schedule.DataShowName
				if schedule.DataEpisodeTitle != "" {
					title += " - " + schedule.DataEpisodeTitle
				}
				mu.Lock()
				res.Programmes = append(res.Programmes, &EPGProgramme{
					Channel: channel.ID,
					Start:   startTime.Format(consts.TIME_FORMAT),
					Stop:    endTime.Format(consts.TIME_FORMAT),
					Title:   title,
					Desc:    schedule.DataDescription,
				})
				mu.Unlock()
			}
		}(channel)
	}
	wg.Wait()
	if len(res.Channels) == 0 {
		return fmt.Errorf("no valid channels found for EPG")
	}
	data, err := xml.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	tempFile := "epg.xml.tmp"
	err = os.WriteFile(tempFile, append([]byte(xml.Header), data...), 0644)
	if err != nil {
		fmt.Println("Error writing XML:", err)
		return err
	}
	err = os.Rename(tempFile, "epg.xml")
	if err != nil {
		fmt.Println("Error renaming XML:", err)
		return err
	}
	return nil
}

func LoadYaml() ([]*tv.TheTV, error) {
	_, err := os.Stat("tvList.yaml")
	if os.IsNotExist(err) {
		fmt.Println("tvList.yaml not found, returning empty list.")
		return []*tv.TheTV{}, nil
	} else if err != nil {
		fmt.Println("Error checking file:", err)
		return nil, err
	}
	data, err := os.ReadFile("tvList.yaml")
	if err != nil {
		fmt.Println("Error reading YAML file:", err)
		return nil, err
	}
	var tvList []*tv.TheTV
	err = yaml.Unmarshal(data, &tvList)
	if err != nil {
		fmt.Println("Error unmarshaling YAML:", err)
		return nil, err
	}
	if tvList == nil {
		return []*tv.TheTV{}, nil
	}
	return tvList, nil
}
