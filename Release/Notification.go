
package notification

import (
	"bytes"
	"cloud.google.com/go/storage"
	"context"
	"encoding/json"
	"github.com/PuerkitoBio/goquery"
	"github.com/microcosm-cc/bluemonday"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type PubSubMessage struct {
	Data []byte `json:"data"`
}

type NewsInfo struct {
	Date    string `json:"date"`
	Type    string `json:"type"`
	Content string `json:"content"`
	Url     string `json:"url"`
}

func SendNotification(_ context.Context,_ PubSubMessage) error{

	bucketName := os.Getenv("Bucket_Name")

	p := bluemonday.NewPolicy().AllowElements("br")

	newData := make([]NewsInfo, 8, 8)

	res, err := http.Get("http://www.kisarazu.ac.jp/")

	if err != nil || res == nil {
		log.Fatal(err)
		return err
	}

	defer res.Body.Close()
	doc, _ := goquery.NewDocumentFromReader(res.Body)

	for i := 1; i <= 8; i++ {
		find := "div.content_wrap:nth-child(1) > dl:nth-child(" + strconv.Itoa(i) + ")"

		src := doc.Find(find)
		data1, _ := src.Find("dt:nth-child(1)").Html()
		arr := strings.Split(p.Sanitize(data1), "<br/>")

		data2, _ := src.Find("dd:nth-child(2) > a:nth-child(1)").Html()
		content := p.Sanitize(data2)

		url := src.Find("dd:nth-child(2) > a:nth-child(1)").AttrOr("href", "Default")
		newData[i-1] = NewsInfo{Date: arr[0], Type: arr[1], Content: content, Url: url}

	}

	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, time.Second*50)

	defer cancel()
	client, err := storage.NewClient(ctx)

	if err != nil {
		log.Fatal(err)
		return err
	}

	rc, err := client.Bucket(bucketName).Object("data.json").NewReader(ctx)

	if err != nil {
		log.Fatal(err)
		return err
	}

	defer rc.Close()

	oldjson, err := ioutil.ReadAll(rc)
	if err != nil {
		log.Fatal(err)
		return err
	}

	var oldData []NewsInfo

	err = json.Unmarshal(oldjson, &oldData)

	if err != nil {
		log.Fatal(err)
		return err
	}

	var checkList []NewsInfo

	for i := 0; i < 8; i++ {
		checkData := newData[i]
		var check = false
		for j := 0; j < 8; j++ {
			if checkData == oldData[j] {
				check = true
			}
		}

		if !check {
			checkList = append(checkList, newData[i])
		}
	}

	for _, v := range checkList {

		lineStr := `{"messages":[{"type":"text","text":"` + v.Type + `\n` + v.Content + `\n` + v.Url + `"}]}`

		LINE := "https://api.line.me/v2/bot/message/broadcast"
		secret_Key := os.Getenv("LINE_Token_Key")
		req, err := http.NewRequest(
			"POST",
			LINE,
			bytes.NewBuffer([]byte(lineStr)),
		)

		if err != nil {
			log.Fatal(err)
			return err
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+secret_Key)

		client := &http.Client{}

		resp, err := client.Do(req)
		log.Println(resp)


		if err != nil {
			log.Fatal(err)
			return err
		}

		resp.Body.Close()
	}

	jsonData, err := json.MarshalIndent(newData, "", "  ")

	wc := client.Bucket(bucketName).Object("data.json").NewWriter(ctx)

	tmpFile, err := os.Create("/tmp/data.json")
	if err != nil {
		log.Fatal(err)
		return err
	}

	if err := ioutil.WriteFile("/tmp/data.json", jsonData, os.ModePerm); err != nil {
		log.Fatal(err)
		return err
	}

	if _, err := io.Copy(wc, tmpFile); err != nil {
		log.Fatal(err)
		return err
	}

	if err := wc.Close(); err != nil {
		log.Fatal(err)
		return err
	}

	return nil

}
