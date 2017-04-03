package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/gorilla/sessions"
	"github.com/justinas/alice"
)

const AppName = "gjvote"

// SiteData is stuff that stays the same
type siteData struct {
	Title       string `json:"title"`
	Port        int    `json:"port"`
	SessionName string `json:"session"`
	ServerDir   string `json:"dir"`
	DevMode     bool   `json:"devmode"`
	DB          string `json:"db"`
}

// pageData is stuff that changes per request
type pageData struct {
	Site           *siteData
	Title          string
	HideTitleImage bool
	SubTitle       string
	Stylesheets    []string
	HeaderScripts  []string
	Scripts        []string
	FlashMessage   string
	FlashClass     string
	LoggedIn       bool
	Menu           []menuItem
	BottomMenu     []menuItem
	session        *pageSession

	TemplateData interface{}
}

type menuItem struct {
	Label    string
	Location string
	Icon     string
}

var sessionSecret = "JCOP5e8ohkTcOzcSMe74"

var sessionStore = sessions.NewCookieStore([]byte(sessionSecret))
var site *siteData
var r *mux.Router
var configFile string

func main() {
	configFile = "./config.json"
	loadConfig()
	saveConfig()
	initialize()

	r = mux.NewRouter()
	r.StrictSlash(true)

	s := http.StripPrefix("/assets/", http.FileServer(http.Dir(site.ServerDir+"assets/")))
	r.PathPrefix("/assets/").Handler(s)

	// Public Subrouter
	pub := r.PathPrefix("/").Subrouter()
	pub.HandleFunc("/", handleMain)

	// API Subrouter
	//api := r.PathPrefix("/api").Subtrouter()

	// Admin Subrouter
	admin := r.PathPrefix("/admin").Subrouter()
	admin.HandleFunc("/", handleAdmin)
	admin.HandleFunc("/dologin", handleAdminDoLogin)
	admin.HandleFunc("/dologout", handleAdminDoLogout)
	admin.HandleFunc("/{category}", handleAdmin)
	admin.HandleFunc("/{category}/{id}", handleAdmin)
	admin.HandleFunc("/{category}/{id}/{function}", handleAdmin)

	http.Handle("/", r)

	chain := alice.New(loggingHandler).Then(r)

	fmt.Printf("Listening on port %d\n", site.Port)
	log.Fatal(http.ListenAndServe("127.0.0.1:"+strconv.Itoa(site.Port), chain))
}

func loadConfig() {
	site = new(siteData)
	// Defaults:
	site.Title = "ICT GameJam"
	site.Port = 8080
	site.SessionName = "ict-gamejam"
	site.ServerDir = "./"
	site.DevMode = false
	site.DB = AppName + ".db"

	jsonInp, err := ioutil.ReadFile(configFile)
	if err == nil {
		assertError(json.Unmarshal(jsonInp, &site))
	}
}

func saveConfig() {
	var jsonInp []byte
	jsonInp, _ = json.Marshal(site)
	assertError(ioutil.WriteFile(configFile, jsonInp, 0644))
}

func initialize() {
	// Check if the database has been created
	assertError(openDatabase())

	if !dbHasUser() {
		reader := bufio.NewReader(os.Stdin)
		fmt.Println("Create new Admin user")
		fmt.Print("Email: ")
		email, _ := reader.ReadString('\n')
		email = strings.TrimSpace(email)
		var pw1, pw2 []byte
		for string(pw1) != string(pw2) || string(pw1) == "" {
			fmt.Print("Password: ")
			pw1, _ = terminal.ReadPassword(0)
			fmt.Println("")
			fmt.Print("Repeat Password: ")
			pw2, _ = terminal.ReadPassword(0)
			fmt.Println("")
			if string(pw1) != string(pw2) {
				fmt.Println("Entered Passwords don't match!")
			}
		}
		assertError(dbUpdateUserPassword(email, string(pw1)))
	}
}

func loggingHandler(h http.Handler) http.Handler {
	return handlers.LoggingHandler(os.Stdout, h)
}

func (p *pageData) show(tmplName string, w http.ResponseWriter) error {
	for _, tmpl := range []string{
		"htmlheader.html",
		"admin-menu.html",
		"header.html",
		tmplName,
		"footer.html",
		"htmlfooter.html",
	} {
		if err := outputTemplate(tmpl, p, w); err != nil {
			fmt.Printf("%s\n", err)
			return err
		}
	}
	return nil
}

// outputTemplate
// Spit out a template
func outputTemplate(tmplName string, tmplData interface{}, w http.ResponseWriter) error {
	_, err := os.Stat("templates/" + tmplName)
	if err == nil {
		t := template.New(tmplName)
		t, _ = t.ParseFiles("templates/" + tmplName)
		return t.Execute(w, tmplData)
	}
	return fmt.Errorf("WebServer: Cannot load template (templates/%s): File not found", tmplName)
}

// redirect can be used only for GET redirects
func redirect(url string, w http.ResponseWriter, req *http.Request) {
	http.Redirect(w, req, url, 303)
}

func printOutput(out string) {
	if site.DevMode {
		fmt.Print(out)
	}
}

func assertError(err error) {
	if err != nil {
		panic(err)
	}
}
