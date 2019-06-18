package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/bdwilliams/go-jsonify/jsonify"
	"github.com/dustin/go-humanize"
	_ "github.com/mattn/go-sqlite3"
)

type disk struct {
	Mount             string
	Size              uint64
	SizeReadable      string
	Available         uint64
	AvailableReadable string
	Used              uint64
	UsedReadable      string
	Percentage        uint8
}

var db *sql.DB
var diskInfo []disk

func handler(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.ParseFiles("html/main.html"))
	t.Execute(w, diskInfo)
}

func graphHandler(w http.ResponseWriter, r *http.Request) {
	diskName := strings.Replace(r.URL.Path[len("/graph/"):], "@", "/", -1)
	for _, disk := range diskInfo {
		if disk.Mount == diskName {
			fmt.Println(diskName)
			t := template.Must(template.ParseFiles("html/graph.html"))
			t.Execute(w, disk)
			return
		}
	}

	w.WriteHeader(http.StatusBadRequest)
}

func dataHandler(w http.ResponseWriter, r *http.Request) {
	diskName := strings.Replace(r.URL.Path[len("/data/"):], "@", "/", -1)
	for _, disk := range diskInfo {
		if disk.Mount == diskName {
			rows, err := db.Query(fmt.Sprintf("SELECT time,bytes from logs WHERE mount='%s'", disk.Mount))
			checkErr(err)
			defer rows.Close()

			json := jsonify.Jsonify(rows)
			jsonString := "["
			for _, line := range json {
				jsonString += line
			}
			jsonString += "]"

			w.Write([]byte(jsonString))
			return
		}
	}

	w.WriteHeader(http.StatusBadRequest)
}

func main() {
	// how often to log (seconds)
	var logInterval = 900

	watchList := getWatchList()
	updateDiskInfo(watchList)

	var err error
	db, err = sql.Open("sqlite3", "./diskInfo.db")
	checkErr(err)

	go logDiskInfo(logInterval, watchList)

	http.HandleFunc("/", handler)
	http.HandleFunc("/graph/", graphHandler)
	http.HandleFunc("/data/", dataHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.ListenAndServe(":8192", nil)
}

// updates the disk information
func updateDiskInfo(watchList []string) {
	var dInfo []disk

	for _, dir := range watchList {
		var stat syscall.Statfs_t
		syscall.Statfs(dir, &stat)

		bSize := uint64(stat.Bsize)

		size := uint64(stat.Blocks * bSize)
		sizeReadable := humanize.IBytes(size)
		available := uint64(stat.Bavail * bSize)
		availableReadable := humanize.IBytes(available)
		used := size - available
		usedReadable := humanize.IBytes(used)
		percentage := uint8(math.Round((float64(used) / float64(size)) * 100))

		dInfo = append(dInfo, disk{dir, size, sizeReadable, available, availableReadable, used, usedReadable, percentage})
	}
	diskInfo = dInfo
}

func logDiskInfo(logInterval int, watchList []string) {
	// make sure the log table exists
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS logs (mount VARCHAR(100), time TIMESTAMP, bytes BIGINT)")
	checkErr(err)

	var lastLogTimeStampNull sql.NullInt64
	err = db.QueryRow("SELECT MAX(time) FROM logs").Scan(&lastLogTimeStampNull)
	checkErr(err)

	// if there is previous data
	if lastLogTimeStampNull.Valid {
		lastLogTimeStamp, err := lastLogTimeStampNull.Value()
		checkErr(err)

		lastLogTime := time.Unix(lastLogTimeStamp.(int64), 0)
		// check if a log is due
		if time.Now().Sub(lastLogTime).Seconds() < float64(logInterval) {
			// calculate time until next log is due
			timeTillLog := lastLogTime.Add(time.Duration(logInterval) * time.Second).Sub(time.Now())
			// sleep until then
			time.Sleep(timeTillLog)
		} else {
			//calculate time next log time that fits interval
			timeTillLog := float64(logInterval) - math.Mod(time.Now().Sub(lastLogTime).Seconds(), float64(logInterval))
			// sleep until then
			time.Sleep(time.Duration(timeTillLog) * time.Second)
		}
	}

	// keep logging every (logInterval) seconds until the program stops
	for {
		go func(watchList []string) {
			// update diskInfo before every log
			updateDiskInfo(watchList)

			for _, disk := range diskInfo {
				_, err := db.Exec("INSERT INTO logs VALUES ('" + disk.Mount + "', strftime('%s','now'),'" + strconv.FormatUint(disk.Available, 10) + "')")
				checkErr(err)
			}
		}(watchList)

		time.Sleep(time.Duration(logInterval) * time.Second)
	}
}

// panic if there is an error
func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

// reads watchlist from file to get mountpoints to check
func getWatchList() []string {
	data, err := ioutil.ReadFile("watchlist.txt")
	if err == nil {
		lines := strings.Split(string(data), "\n")

		var wl []string

		for _, line := range lines {
			// ignore comment lines
			if !strings.HasPrefix(line, "#") {
				wl = append(wl, strings.TrimSpace(line))
			}
		}

		return wl
	}
	panic(err)
}
