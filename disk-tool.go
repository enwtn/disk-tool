package main

import (
	"html/template"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"github.com/dustin/go-humanize"
)

type disk struct {
	Name              string
	Size              uint64
	SizeReadable      string
	Used              uint64
	UsedReadable      string
	Available         uint64
	AvailableReadable string
	Percentage        uint64
	Mount             string
}

var diskInfo []disk

func handler(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.ParseFiles("html/main.html"))
	t.Execute(w, diskInfo)
}

func main() {
	updateDiskInfo()

	http.HandleFunc("/", handler)
	http.ListenAndServe(":8192", nil)
}

func updateDiskInfo() {
	c := exec.Command("df")
	out, _ := c.Output()

	lines := strings.Split(string(out), "\n")

	var diskInfoTemp []disk

	for _, line := range lines[1 : len(lines)-1] {
		fields := strings.Fields(line)

		// sizes are in 1k blocks
		name := fields[0]
		size, _ := strconv.ParseUint(fields[1], 10, 64)
		used, _ := strconv.ParseUint(fields[2], 10, 64)
		availible, _ := strconv.ParseUint(fields[3], 10, 64)
		percentage, _ := strconv.ParseUint(strings.Trim(fields[4], "%"), 10, 8)
		mount := fields[5]

		diskInfoTemp = append(diskInfoTemp,
			disk{name, size * 1024, humanize.IBytes(size * 1024),
				used * 1024, humanize.IBytes(used * 1024),
				availible * 1024, humanize.IBytes(availible * 1024),
				percentage, mount})
	}
	diskInfo = diskInfoTemp
}
