package lib

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"code.google.com/p/goauth2/oauth"
)

type LocalServerConfig struct {
	Port    int
	Timeout int
	OS      string
}

type RedirectResult struct {
	Code string
	Err  error
}

type Redirect struct {
	Result      chan RedirectResult
	ServerStart chan bool
	ServerStop  chan bool
	Listener    net.Listener
}

// 各種OSでのブラウザ起動コマンドとURLのエスケープコード置換文字列
type OpenBrowser struct {
	EscapeAnd string
	arg       []string
}

var openBrowser = map[string]OpenBrowser{
	"windows": {`&`, []string{"cmd", "/c", "start"}},
	"darwin":  {`&`, []string{"open", "-a", "safari"}},
	"test1":   {`&`, []string{"echo", "", ""}},
	"test2":   {`&`, []string{"fugafuga", "", ""}},
}

func NewRedirect(result chan RedirectResult) *Redirect {
	return &Redirect{result, make(chan bool, 1), make(chan bool, 1), nil}
}

type AuthToken interface {
	GetTokenCache() error
	GetAuthCodeURL() string
	GetAuthToken(string) error
}

type GoogleToken struct {
	Transport *oauth.Transport
}

// テストしやすいようにAuth系APIを隠蔽する
func (this *GoogleToken) GetTokenCache() error {
	_, err := this.Transport.Config.TokenCache.Token()
	return err
}
func (this *GoogleToken) GetAuthCodeURL() string {
	return this.Transport.Config.AuthCodeURL("")
}
func (this *GoogleToken) GetAuthToken(code string) error {
	_, err := this.Transport.Exchange(code)
	return err
}

// アクセストークンを取得
func GoogleOauth(transport AuthToken, localServerConfig LocalServerConfig) (err error) {

	// キャッシュからトークンファイルを取得
	err = transport.GetTokenCache()
	if err == nil {
		return
	}
	url := transport.GetAuthCodeURL()
	code, err := getAuthCode(url, localServerConfig)
	if err != nil {
		err = fmt.Errorf("Error getAuthCode: %#v", err)
		return
	}
	// 認証トークンを取得する。（取得後、キャッシュへ）
	err = transport.GetAuthToken(code)
	if err != nil {
		err = fmt.Errorf("Exchange: %#v", err)
	}
	return
}

// アクセスコード取得
func (this *Redirect) GetCode(w http.ResponseWriter, r *http.Request) {
	//defer this.Listener.Stop()
	code := r.URL.Query().Get("code")

	if code == "" {
		fmt.Fprintf(w, `Erorr`)
		this.Result <- RedirectResult{Err: fmt.Errorf("codeを取得できませんでした。")}
		return
	}

	fmt.Fprintf(w, `<!doctype html> <html lang="ja"> <head> <meta charset="utf-8"> </head>
            <body onload="window.open('about:blank','_self').close();">ブラウザが自動で閉じない場合は手動で閉じてください。</body>
            </html> `)
	this.Result <- RedirectResult{Code: code}
}

// localhostのhttpサーバー
func (this *Redirect) Server(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", this.GetCode)
	host := fmt.Sprintf("localhost:%d", port)
	fmt.Printf("Start Listen: %s\n", host)
	var err error
	this.Listener, err = net.Listen("tcp", host)
	if err != nil {
		this.Result <- RedirectResult{Err: err}
		return
	}
	server := http.Server{}
	server.Handler = mux
	go server.Serve(this.Listener)
	this.ServerStart <- true
	<-this.ServerStop
	this.Listener.Close()
	this.Result <- RedirectResult{Err: err}
	return
}
func (this *Redirect) Stop() {
	this.ServerStop <- true
}

// サーバー起動 -> ブラウザ起動 -> コード取得
func getAuthCode(url string, localServerConfig LocalServerConfig) (string, error) {

	var cmd *exec.Cmd

	//os := runtime.GOOS
	os := localServerConfig.OS
	var browser *OpenBrowser
	for key, value := range openBrowser {
		if os == key {
			browser = &value
			break
		}
	}
	if browser == nil {
		return "", fmt.Errorf("まだ未対応です・・・\n%s\n", url)
	}

	redirect := NewRedirect(make(chan RedirectResult, 1))
	go redirect.Server(localServerConfig.Port)

	// set redirect timeout
	redirectTimeout := time.After(time.Duration(localServerConfig.Timeout) * time.Second)
	<-redirect.ServerStart

	url = strings.Replace(url, "&", browser.EscapeAnd, -1)
	// ブラウザ起動

	//fmt.Printf("%v %v %v %v", browser.arg[0], browser.arg[1], browser.arg[2], url)
	cmd = exec.Command(browser.arg[0], browser.arg[1], browser.arg[2], url)
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("Error:  start browser: %v, browser: %v\n", err, browser)
	}

	defer redirect.Stop()
	var result RedirectResult

	select {
	case result = <-redirect.Result:
		//ブラウザ側の応答があればなにもしない
	case <-redirectTimeout:
		// タイムアウト
		return "", fmt.Errorf("リダイレクト待ち時間がタイムアウトしました")
	}

	if result.Err != nil {
		return "", fmt.Errorf("Error: リダイレクト: %v\n", result.Err)
	}

	fmt.Printf("code: %v\n", result.Code)

	return result.Code, nil
}
