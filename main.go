/*
	Bytesupply.com - Web Server Pages App
	=====================================

	Complete documentation and user guides are available here:
	https://https://github.com/yveshoebeke/bytesupply/blob/master/README.md

	@author	yves.hoebeke@accds.com - 1011001.1110110.1100101.1110011

	@version 1.0.0

	(c) 2020 - Bytesupply, LLC - All Rights Reserved.
*/

package main

/* System libraries */
import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"text/template"
	"time"

	"bytesupply.com/googleapi"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
)

var (
	/* Extract env variables */
	staticLocation = os.Getenv("BS_STATIC_LOCATION")
	logFile        = os.Getenv("BS_LOGFILE")
	msgFile        = os.Getenv("BS_MSGFILE")
	serverPort     = os.Getenv("BS_SERVER_PORT")
	dbHost         = os.Getenv("BS_MYSQL_HOST")
	dbPort         = os.Getenv("BS_MYSQL_PORT")
	dbUser         = os.Getenv("BS_MYSQL_USERNAME")
	dbPassword     = os.Getenv("BS_MYSQL_PASSWORD")
	dbDatabase     = os.Getenv("BS_MYSQL_DB")

	/* templating */
	tmpl    = template.Must(template.New("").Funcs(funcMap).ParseGlob(staticLocation + "/templ/*"))
	funcMap = template.FuncMap{
		"hasHTTP": func(myUrl string) string {
			if strings.Contains(myUrl, "://") {
				return myUrl
			}

			return "https://" + myUrl
		},
	}
)

// User - info */
type User struct {
	Username  string    `json:"username"`
	Password  string    `json:"password"`
	Realname  string    `json:"realname"`
	Title     string    `json:"title"`
	LoginTime time.Time `json:"logintime"`
}

// Data - database structure */
type Data struct {
	ReqType   string    `json:"reqtype"`
	ReqCmd    string    `json:"reqcmd"`
	Timestamp time.Time `json:"Timestamp"`
}

// App - application structure */
type App struct {
	log   *log.Logger
	lfile *os.File
	user  User
	db    *sql.DB
}

// GetIP - IP address retriever */
func GetIP(r *http.Request) string {
	forwarded := r.Header.Get("X-FORWARD-FOR")
	if forwarded != "" {
		return forwarded
	}

	return r.RemoteAddr
}

/* Routers */
func (app *App) homepage(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, staticLocation+"/html/index.html")
}

func (app *App) home(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, staticLocation+"/html/home.html")
}

func (app *App) company(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, staticLocation+"/html/company.html")
}

func (app *App) staff(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, staticLocation+"/html/staff.html")
}

func (app *App) history(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, staticLocation+"/html/history.html")
}

func (app *App) contactus(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		http.ServeFile(w, r, staticLocation+"/html/contactus.html")
	} else if r.Method == http.MethodPost {
		// process contact us info
		type MsgStatus struct {
			ValidToSend bool   `json:"validtosend"`
			Name        string `json:"name"`
		}

		r.ParseForm()

		var validToRecord = false

		if r.FormValue("validEntry") == "false" {
			validToRecord = false
		} else {
			validToRecord = true
			// Validate (name, email and message are mandatory)
			for varName, varValue := range r.Form {
				switch varName {
				case "contactName":
				case "contactEmail":
				case "contactMessage":
					if len(varValue[0]) == 0 {
						validToRecord = false
					}
					break
				default:
					break
				}
			}

			if validToRecord {
				sqlStatement := `INSERT INTO messages (user, name,company,email,phone,message) VALUES (?, ?, ?, ?, ?, ?)`
				_, err := app.db.Exec(sqlStatement, app.user.Username, r.FormValue("contactName"), r.FormValue("contactCompany"), r.FormValue("contactEmail"), r.FormValue("contactPhone"), r.FormValue("contactMessage"))
				if err != nil {
					app.log.Println("ContactUs INSERT sql err:", err.Error())
				}
			}
		}

		msgStatus := MsgStatus{ValidToSend: validToRecord, Name: r.FormValue("contactName")}
		tmpl.ExecuteTemplate(w, "contactussent.gotmpl.html", msgStatus)
	}
}

func (app *App) search(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	searchKey := url.QueryEscape(r.FormValue("searchKey"))

	if len(searchKey) != 0 {
		searchResults, err := googleapi.GetSearchResults(searchKey)
		if err != nil {
			app.log.Println("Google API Err:", err)
		} else {
			tmpl.ExecuteTemplate(w, "search.gotmpl.html", searchResults)
		}
	} else {
		http.Redirect(w, r, r.FormValue("referer"), http.StatusSeeOther)
	}
}

func (app *App) expertise(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, staticLocation+"/html/expertise.html")
}

func (app *App) terms(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, staticLocation+"/html/terms.html")
}

func (app *App) privacy(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, staticLocation+"/html/privacy.html")
}

func (app *App) products(w http.ResponseWriter, r *http.Request) {
	type Item struct {
		ItemToShow string `json:"itemtoshow"`
	}
	item := Item{ItemToShow: "all"}
	tmpl.ExecuteTemplate(w, "product.gotmpl.html", item)
}

func (app *App) product(w http.ResponseWriter, r *http.Request) {
	type Item struct {
		ItemToShow string `json:"itemtoshow"`
	}
	vars := mux.Vars(r)
	itemtoshow := vars["item"]
	item := Item{ItemToShow: itemtoshow}
	app.log.Println("Item:", vars["item"])
	tmpl.ExecuteTemplate(w, "product.gotmpl.html", item)
}

func (app *App) test(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	app.log.Println("Object:", vars["object"])
	http.ServeFile(w, r, staticLocation+"/html/"+vars["object"]+".html")
}

func (app *App) getlog(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<p style=\"color:blue;\"><a href=\"/home\">Bytesupply</a></p><p>Access log</p>")

	logfile, err := os.Open(logFile)
	if err != nil {
		fmt.Fprintf(w, "<p style=\"color:blue;\">%s failed to open: %s</p>", logFile, err)
	} else {
		scanner := bufio.NewScanner(logfile)
		scanner.Split(bufio.ScanLines)

		fmt.Fprintf(w, "<ul>")
		for scanner.Scan() {
			fmt.Fprintf(w, "<li>%s</li>", scanner.Text())
		}
		fmt.Fprintf(w, "</ul>")
		logfile.Close()
	}
}

func (app *App) registerUser(r *http.Request) error {
	app.user.Username = r.PostFormValue("username")
	app.user.Password = r.PostFormValue("password")
	app.user.Realname = "Yves Hoebeke"
	app.user.Title = "Owner"
	app.user.LoginTime = time.Now()
	app.log.Printf("Registering user %s as %s with username: %s and password: %s", app.user.Realname, app.user.Title, app.user.Username, app.user.Password)

	return nil
}

func (app *App) api(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	version := vars["version"]
	request := vars["request"]

	app.log.Println("@api with version:", version, "and request:", request)

	switch version {
	default:
	case "v1":
		switch request {
		case "qTurHm":
			app.qTurHm(w, r)
		case "request":
			app.request(w, r)
		}
	}
}

func (app *App) qTurHm(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		http.ServeFile(w, r, staticLocation+"/html/contactus.html")
	} else if r.Method == http.MethodPost {
		type Target struct {
			Top    int `json:"top"`
			Left   int `json:"left"`
			Width  int `json:"width"`
			Height int `json:"height"`
		}

		type Move struct {
			T int `json:"t"`
			X int `json:"x"`
			Y int `json:"y"`
		}

		type QTurHm struct {
			Key           string `json:"userkey"`
			TimeCreated   int    `json:"timestamp"`
			ResultContent string `json:"resultcontent"`
			URL           string `json:"origURL"`
			Mobile        bool   `json:"mobile"`
			Target        Target `json:"target"`
			Receiver      string `json:"receiver"`
			SampleCount   int    `json:"samples"`
			Moves         []Move `json:"moves"`
		}

		var q QTurHm

		// Try to decode the request body into the struct.
		err := json.NewDecoder(r.Body).Decode(&q)
		if err != nil {
			app.log.Println("API error (qTurHm):", err.Error())
			return
		}

		app.log.Printf("%v", q)
		app.log.Printf("Key: %s Time: %d", q.Key, q.TimeCreated)
		rfn := q.Key + "_" + strconv.Itoa(q.TimeCreated)
		app.log.Printf("Result File Name: %s should be: %s", rfn, q.ResultContent)

		res := []byte("8")
		werr := ioutil.WriteFile("/go/bin/data/qTurHm/"+rfn, res, 0644)
		if werr != nil {
			app.log.Printf("Error writing result file /go/bin/data/qTurHm/%s: %v", rfn, werr)
		}
	}
}

func (app *App) request(w http.ResponseWriter, r *http.Request) {
	reqBody, err := ioutil.ReadAll(r.Body)
	if err != nil {
		app.log.Println("Error parsing Body:", err)
	}
	var data Data
	json.Unmarshal(reqBody, &data)
	data.Timestamp = time.Now()

	json.NewEncoder(w).Encode(data)

	app.log.Printf("Request command received: %s", data.ReqType)
}

/* Middleware */
func (app *App) inMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		app.log.Printf("User: %s | URL: %s | Method: %s | IP: %s", app.user.Username, r.URL.Path, r.Method, GetIP(r))
		next.ServeHTTP(w, r)
	})
}

/*
       ^ ^
      (o O)
 ___oOO(.)OOo___
 _______________

 ************************************
 *	Execution start point!!!!!!!!!	*
 *	Structure and Methods 			*
 *	- Setup and start of app.		*
 *	- Serve and Listen.				*
 ************************************

*/
func init() {
}

func main() {
	// Logging
	logger := log.New()
	logger.SetFormatter(&log.TextFormatter{
		ForceColors:     false,
		FullTimestamp:   true,
		TimestampFormat: time.RFC822,
	})
	logger.SetLevel(log.InfoLevel)

	// log file set up
	lf, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Error opening logfile: %s -> %v", logFile, err)
	}
	defer lf.Close()

	mw := io.MultiWriter(os.Stdout, lf)
	logger.SetOutput(mw)

	// mysql connectivity
	dbConnectData := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dbUser, dbPassword, dbHost, dbPort, dbDatabase)
	db, err := sql.Open("mysql", dbConnectData)
	if err != nil {
		fmt.Println("db connect issue:", err.Error())
	}
	defer db.Close()

	// Initial user data (before actual login)
	user := User{
		Username:  "WWW",
		Password:  "",
		Realname:  "Visitor",
		Title:     "User",
		LoginTime: time.Now(),
	}

	// Set app values
	app := &App{
		log:   logger,
		lfile: lf,
		user:  user,
		db:    db,
	}

	app.log.Println("Starting service.")

	/* Routers definitions */
	r := mux.NewRouter()

	/* Middleware */
	r.Use(app.inMiddleWare)

	/* Allow static content */
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir(staticLocation))))

	/* Handlers */
	r.HandleFunc("/", app.homepage).Methods(http.MethodGet)
	r.HandleFunc("/company", app.company).Methods(http.MethodGet)
	r.HandleFunc("/home", app.home).Methods(http.MethodGet)
	r.HandleFunc("/staff", app.staff).Methods(http.MethodGet)
	r.HandleFunc("/history", app.history).Methods(http.MethodGet)
	r.HandleFunc("/contactus", app.contactus).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/search", app.search).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/expertise", app.expertise).Methods(http.MethodGet)
	r.HandleFunc("/terms", app.terms).Methods(http.MethodGet)
	r.HandleFunc("/privacy", app.privacy).Methods(http.MethodGet)
	r.HandleFunc("/product/{item:[a-zA-Z]+}", app.product).Methods(http.MethodGet)
	r.HandleFunc("/products", app.products).Methods(http.MethodGet)
	r.HandleFunc("/getlog", app.getlog).Methods(http.MethodGet)
	r.HandleFunc("/request", app.request).Methods("POST")
	r.HandleFunc("/test/{object:[a-z]+}", app.test).Methods(http.MethodGet, http.MethodPost)
	r.HandleFunc("/api/{version:[a-z0-9]+}/{request:[a-zA-Z]+}", app.api).Methods(http.MethodGet, http.MethodPost)

	/* Server setup and start */
	BytesupplyServer := &http.Server{
		Handler:      handlers.LoggingHandler(os.Stdout, r),
		Addr:         serverPort,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}

	/*
	**************************************
	* Setup and initialization completed *
	*                                    *
	*         Launch the server!         *
	**************************************
	 */
	app.log.Fatal(BytesupplyServer.ListenAndServe())

	/*
		****************************************************
		POST request test:

		curl --header "Content-Type: application/json" \
		--request POST \
		--data '{"reqtype":"test", "reqcmd":"Here is some requested data"}' \
		https://bytesupply.com/api/v1/request

		****************************************************
	*/
}
