package tv

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sync"
	"sync/atomic"
	"thetv-apg/consts"
	"time"

	"github.com/PuerkitoBio/goquery"
	"gopkg.in/yaml.v3"
)

type TheTV struct {
	Name string `json:"name"`
	Path string `json:"path"`
	ID   string `json:"id"`
}

type TVSchedule struct {
	Title        string `json:"title"`
	StartTime    int64  `json:"startTime"`
	EndTime      int64  `json:"endTime"`
	EpisodeTitle string `json:"episodeTitle"`
}

var httpClient = &http.Client{
	Timeout: 20 * time.Second,
}

func fetchUrl(url string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", consts.UA)

	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode != http.StatusOK {
		res.Body.Close()
		return nil, fmt.Errorf("request failed: %s", res.Status)
	}
	return res, nil
}

func getTVPathList() ([]*TheTV, error) {
	res, err := fetchUrl(fmt.Sprintf("%s/tv", consts.THETV_URL), nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return nil, err
	}
	var tvPathList []*TheTV
	doc.Find(".list-group.list-group-numbered a").Each(func(i int, s *goquery.Selection) {
		link, _ := s.Attr("href")
		tvPathList = append(tvPathList, &TheTV{
			Name: s.Text(),
			Path: link,
		})
	})
	return tvPathList, nil
}

func getTVId(tv *TheTV) (string, error) {
	url := consts.THETV_URL + tv.Path
	res, err := fetchUrl(url, nil)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	reg := regexp.MustCompile(`jsonUrl = \"https://thetvapp.to/json/(\d+)\.json\";`)
	matches := reg.FindStringSubmatch(string(bodyBytes))
	if len(matches) < 2 {
		return "", fmt.Errorf("failed to extract TV ID from page")
	}
	return matches[1], nil
}

func getTVList() ([]*TheTV, error) {
	tvPathList, err := getTVPathList()
	if err != nil {
		return nil, err
	}

	var (
		tvList   []*TheTV
		mu       sync.Mutex
		wg       sync.WaitGroup
		errCount int32
	)

	for _, tv := range tvPathList {
		wg.Add(1)
		go func(tv *TheTV) {
			defer wg.Done()
			tvID, err := getTVId(tv)
			if err != nil {
				atomic.AddInt32(&errCount, 1)
				fmt.Printf("Warning: failed to fetch TV ID for %s: %v\n", tv.Name, err)
				return
			}
			mu.Lock()
			tvList = append(tvList, &TheTV{
				Name: tv.Name,
				Path: tv.Path,
				ID:   tvID,
			})
			mu.Unlock()
		}(tv)
	}

	wg.Wait()

	if len(tvList) == 0 {
		return nil, fmt.Errorf("failed to fetch any TV IDs, errors: %d", errCount)
	}
	return tvList, nil
}

func GetTVSchedule(id, path string) ([]*TVSchedule, error) {
	url := fmt.Sprintf("%s/json/%s.json", consts.THETV_URL, id)
	res, err := fetchUrl(url, map[string]string{
		"Referer": fmt.Sprintf("%s/%s", consts.THETV_URL, path),
	})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	var tvSchedule []*TVSchedule
	err = json.NewDecoder(res.Body).Decode(&tvSchedule)
	if err != nil {
		return nil, err
	}
	return tvSchedule, nil
}
func saveTVListToYaml() error {
	tvList, err := getTVList()
	if err != nil {
		return err
	}
	data, err := yaml.Marshal(tvList)
	if err != nil {
		return err
	}
	tempFile := "tvList.yaml.tmp"
	err = os.WriteFile(tempFile, data, 0644)
	if err != nil {
		fmt.Println("Error writing temp YAML file:", err)
		return err
	}
	err = os.Rename(tempFile, "tvList.yaml")
	if err != nil {
		fmt.Println("Error renaming YAML file:", err)
		return err
	}
	fmt.Println("TV list saved successfully: tvList.yaml")
	return nil
}
