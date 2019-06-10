package main

import (
	"html/template"
	"io/ioutil"
	"math"
	"net/http"
	"strings"
	"syscall"

	"github.com/dustin/go-humanize"
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

var watchList []string
var diskInfo []disk

func handler(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.ParseFiles("html/main.html"))
	t.Execute(w, diskInfo)
}

func main() {
	watchList = getWatchList()
	updateDiskInfo()

	http.HandleFunc("/", handler)
	http.ListenAndServe(":8192", nil)
}

// updates the disk information
func updateDiskInfo() {
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

// reads watchlist from file to get mountpoints to check
func getWatchList() []string {
	data, err := ioutil.ReadFile("watchlist.txt")
	if err == nil {
		lines := strings.Split(string(data), "\n")

		var wl []string

		for _, line := range lines {
			if !strings.HasPrefix(line, "#") {
				wl = append(wl, strings.TrimSpace(line))
			}
		}

		return wl
	}
	panic(err)
}
