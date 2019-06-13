package main

import (
	"database/sql"
	"html/template"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
	"syscall"
	"time"

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

var diskInfo []disk

func handler(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.ParseFiles("html/main.html"))
	t.Execute(w, diskInfo)
}

func main() {
	// how often to log (seconds)
	var logInterval = 300

	watchList := getWatchList()
	updateDiskInfo(watchList)

	db, err := sql.Open("sqlite3", "./diskInfo.db")
	checkErr(err)

	go logDiskInfo(db, logInterval)

	http.HandleFunc("/", handler)
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

func logDiskInfo(db *sql.DB, logInterval int) {
	// make sure the log table exists
	_, err := db.Exec("CREATE TABLE IF NOT EXISTS logs (mount VARCHAR(100), time TIMESTAMP, bytes BIGINT)")
	checkErr(err)

	// check if there is any previous data
	rows, err := db.Query("SELECT COUNT(*) FROM logs")
	checkErr(err)

	// if there is previous data then calculate the correct time to log.
	// done so that there isnt random intervals between the logs.
	if rows.Next() {
		rows.Close()
		var lastLogTimeStamp int64
		err = db.QueryRow("SELECT MAX(time) FROM logs").Scan(&lastLogTimeStamp)
		checkErr(err)

		lastLogTime := time.Unix(lastLogTimeStamp, 0)
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
		for _, disk := range diskInfo {
			_, err := db.Exec("INSERT INTO logs VALUES ('" + disk.Mount + "', strftime('%s','now'),'" + strconv.FormatUint(disk.Available, 10) + "')")
			checkErr(err)
		}
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
