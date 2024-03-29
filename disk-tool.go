package main

import (
	"database/sql"
	"html/template"
	"io/ioutil"
	"log"
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
	Mount             string // directory to check
	MountEscaped      string // escaped sirectory (no slashes)
	Size              uint64 // size of disk in bytes
	SizeReadable      string // size of disk in readable format
	Available         uint64 // available disk in bytes
	AvailableReadable string // availble disk in readable format
	Used              uint64 // used disk in bytes
	UsedReadable      string // used disk in readable format
	Percentage        uint8  // percentage of disk used
}

type statsInfo struct {
	Disk          disk   // disk the stats are for
	ChangeMonth   string // change in disk usage past 30 days
	MonthPositive bool   // increase or decrease
	FullMonth     string // how long till disk full calculated from ChangeMonth
	ChangeWeek    string // change in disk usage past 7 days
	WeekPositive  bool   // increase or decrease
	FullWeek      string // how long till disk full calculated from ChangeWeek
	ChangeDay     string // change in disk usage past 24 hours
	DayPositive   bool   // increase or decrease
	FullDay       string // how long till disk full calculated from ChangeDay
}

var db *sql.DB
var diskInfo []disk

// handle home page
func handler(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.ParseFiles("html/main.html"))
	t.Execute(w, diskInfo)
}

// serves statistics pages for disks
func infoHandler(w http.ResponseWriter, r *http.Request) {
	diskName := strings.Replace(r.URL.Path[len("/info/"):], "@", "/", -1)
	for _, disk := range diskInfo {
		if disk.Mount == diskName {
			stats := getStatsInfo(disk)
			t := template.Must(template.ParseFiles("html/info.html"))
			t.Execute(w, stats)
			return
		}
	}

	w.WriteHeader(http.StatusBadRequest)
}

// serves sql data as json array
func dataHandler(w http.ResponseWriter, r *http.Request) {
	diskName := strings.Replace(r.URL.Path[len("/info/"):], "@", "/", -1)

	queries := r.URL.Query()
	since := queries.Get("since")
	var queryString string
	if since != "" {
		queryString = "SELECT time,bytes from logs WHERE mount=? AND time > ?"
	} else {
		queryString = "SELECT time,bytes from logs WHERE mount=?"
	}

	for _, disk := range diskInfo {
		if disk.Mount == diskName {
			rows, err := db.Query(queryString, disk.Mount, since)
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
	var logInterval = 3600

	watchList := getWatchList()
	updateDiskInfo(watchList)

	var err error
	db, err = sql.Open("sqlite3", "./diskInfo.db")
	checkErr(err)

	go logDiskInfo(logInterval, watchList)

	http.HandleFunc("/", handler)
	http.HandleFunc("/info/", infoHandler)
	http.HandleFunc("/data/", dataHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Println("disk-tool running on :8192")
	http.ListenAndServe(":8192", nil)
}

// calculates statistics from current and past disk info
func getStatsInfo(disk disk) statsInfo {
	var times []int64

	timeNow := time.Now().Unix()
	times = append(times, timeNow-2592000) // 30 days ago
	times = append(times, timeNow-604800)  // 7 days ago
	times = append(times, timeNow-86400)   // 1 day ago

	var results []string     // disk space change - readable
	var changeValue []bool   // is change positive or negative?
	var predictions []string // when disk space will run out based on different changes

	query, err := db.Prepare("SELECT bytes FROM logs WHERE time > ? AND mount=? ORDER BY time ASC LIMIT 1")
	checkErr(err)
	defer query.Close()

	for _, t := range times {
		// get the first log after the given time for the given disk
		var bytes uint64
		err := query.QueryRow(strconv.FormatInt(t, 10), disk.Mount).Scan(&bytes)

		if err != nil {
			results = append(results, "ERROR")
			changeValue = append(changeValue, false)
			predictions = append(predictions, "N/A")
		} else {
			if disk.Available >= bytes {
				results = append(results, humanize.IBytes(disk.Available-bytes))
				changeValue = append(changeValue, true)
				predictions = append(predictions, "N/A")
			} else {
				results = append(results, "-"+humanize.IBytes(bytes-disk.Available))
				changeValue = append(changeValue, false)

				spaceChange := int64(bytes - disk.Available)              // disk space difference
				timeChange := timeNow - t                                 // time difference
				bytesPerSecond := spaceChange / timeChange                // bytes used per second on the disk
				secondsTillFull := int64(disk.Available) / bytesPerSecond // seconds until disk is full at current rate

				// parses the number of seconds into a more human readable time format
				var timeTillFull string
				if secondsTillFull > 31556952 {
					timeTillFull = strconv.FormatInt(secondsTillFull/31556952, 10)
					if secondsTillFull > 31556952*2 {
						timeTillFull += " years"
					} else {
						timeTillFull += " year"
					}
				} else {
					timeTillFull = strconv.FormatInt(secondsTillFull/86400, 10)
					if secondsTillFull > 86400*2 {
						timeTillFull += " days"
					} else {
						timeTillFull += " day"
					}
				}

				predictions = append(predictions, timeTillFull)
			}
		}
	}

	s := statsInfo{disk, results[0], changeValue[0], predictions[0], results[1], changeValue[1], predictions[1], results[2], changeValue[2], predictions[2]}
	return s
}

// updates the disk information
func updateDiskInfo(watchList []string) {
	var dInfo []disk

	for _, dir := range watchList {
		mountEscaped := strings.Replace(dir, "/", "@", -1)

		var stat syscall.Statfs_t
		syscall.Statfs(dir, &stat) // get directory info - LINUX ONLY

		bSize := uint64(stat.Bsize) // block size

		size := uint64(stat.Blocks * bSize)                                    // disk size
		sizeReadable := humanize.IBytes(size)                                  // readable disk size
		available := uint64(stat.Bavail * bSize)                               // availible space on disk
		availableReadable := humanize.IBytes(available)                        // readable availble space
		used := size - available                                               // used disk space
		usedReadable := humanize.IBytes(used)                                  // readable used disk space
		percentage := uint8(math.Round((float64(used) / float64(size)) * 100)) // percentage of disk space used

		dInfo = append(dInfo, disk{dir, mountEscaped, size, sizeReadable, available, availableReadable, used, usedReadable, percentage})
	}
	diskInfo = dInfo
}

// periodically records disk usage and current time to database
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
		}
	}

	// keep logging every (logInterval) seconds until the program stops
	for {
		go func(watchList []string) {
			// update diskInfo before every log
			updateDiskInfo(watchList)

			for _, disk := range diskInfo {
				_, err := db.Exec("INSERT INTO logs VALUES(?, strftime('%s','now'), ?)", disk.Mount, strconv.FormatUint(disk.Available, 10))
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
				dir := strings.TrimSpace(line)
				if dir != "" {
					wl = append(wl, dir)
				}
			}
		}

		if len(wl) == 0 {
			log.Fatal("watchlist empty, please add some directories to monitor.")
		}

		return wl
	}
	panic(err)
}
