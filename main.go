package main

import (
	"flag"
	"fmt"
	"log"
	"runtime"

	"code.google.com/p/goauth2/oauth"
	"github.com/masahide/get-cybozu-schedule/lib"
)

func main() {

	flag.Usage = lib.Usage
	flag.Parse()

	if *lib.Version {
		fmt.Printf("%s\n", lib.ShowVersion())
		return
	}

	// ClientID等を読み込む
	config, err := lib.Parse("google.json")
	if err != nil {
		log.Fatalf("Error Server: %v", err)
		return
	}

	port := 3000
	transport := oauth.Transport{
		Config: &oauth.Config{
			ClientId:     config.Installed.ClientID,
			ClientSecret: config.Installed.ClientSecret,
			RedirectURL:  fmt.Sprintf("%s:%d", "http://localhost", port),
			Scope:        "https://www.googleapis.com/auth/calendar",
			AuthURL:      config.Installed.AuthURL,
			TokenURL:     config.Installed.TokenURL,
			TokenCache:   oauth.CacheFile("cache.json"),
		},
	}

	// OAuthを実行
	err = lib.GoogleOauth(&lib.GoogleToken{&transport}, lib.LocalServerConfig{port, 30, runtime.GOOS})
	if err != nil {
		log.Fatalf("Error Server: %v", err)
		return
	}

	// ここからやっとカレンダーAPIを使い始める
	svc, err := calendar.New(transport.Client())
	if err != nil {
		log.Fatalf("Error calendar.New: %v", err)
		return
	}

	// カレンダー一覧を取得
	cl, err := svc.CalendarList.List().Do()
	if err != nil {
		log.Fatalf("Error CalendarList.List(): %v", err)
		return
	}

	fmt.Printf("--- Your calendars ---\n")
	for _, item := range cl.Items {
		fmt.Printf("%# v\n", item)
	}

}
