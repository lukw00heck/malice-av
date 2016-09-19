package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/crackcomm/go-clitable"
	"github.com/maliceio/go-plugin-utils/utils"
	"github.com/parnurzeal/gorequest"
	"github.com/urfave/cli"
	r "gopkg.in/dancannon/gorethink.v2"
)

// Version stores the plugin's version
var Version string

// BuildTime stores the plugin's build time
var BuildTime string

const (
	name     = "fsecure"
	category = "av"
)

type pluginResults struct {
	ID   string      `json:"id" gorethink:"id,omitempty"`
	Data ResultsData `json:"f-secure" gorethink:"f-secure"`
}

// FSecure json object
type FSecure struct {
	Results ResultsData `json:"f-secure"`
}

// ResultsData json object
type ResultsData struct {
	Infected bool        `json:"infected" gorethink:"infected"`
	Results  ScanEngines `json:"results" gorethink:"results"`
	Engine   string      `json:"engine" gorethink:"engine"`
	Database string      `json:"database" gorethink:"database"`
	Updated  string      `json:"updated" gorethink:"updated"`
}

// ScanEngines scan engine results
type ScanEngines struct {
	FSE      string `json:"fse" gorethink:"fse"`
	Aquarius string `json:"aquarius" gorethink:"aquarius"`
}

// ParseFSecureOutput convert fsecure output into ResultsData struct
func ParseFSecureOutput(fsecureout string, path string) (ResultsData, error) {

	// root@70bc84b1553c:/malware# fsav --virus-action1=none eicar.com.txt
	// EVALUATION VERSION - FULLY FUNCTIONAL - FREE TO USE FOR 30 DAYS.
	// To purchase license, please check http://www.F-Secure.com/purchase/
	//
	// F-Secure Anti-Virus CLI version 1.0  build 0060
	//
	// Scan started at Mon Aug 22 02:43:50 2016
	// Database version: 2016-08-22_01
	//
	// eicar.com.txt: Infected: EICAR_Test_File [FSE]
	// eicar.com.txt: Infected: EICAR-Test-File (not a virus) [Aquarius]
	//
	// Scan ended at Mon Aug 22 02:43:50 2016
	// 1 file scanned
	// 1 file infected

	log.Debugln(fsecureout)

	version, database := getFSecureVersion()

	fsecure := ResultsData{
		Infected: false,
		Engine:   version,
		Database: database,
		Updated:  getUpdatedDate(),
	}

	lines := strings.Split(fsecureout, "\n")

	for _, line := range lines {
		if strings.Contains(line, "Infected:") && strings.Contains(line, "[FSE]") {
			fsecure.Infected = true
			parts := strings.Split(line, "Infected:")
			fsecure.Results.FSE = strings.TrimSuffix(parts[1], "[FSE]")
		}
		if strings.Contains(line, "Infected:") && strings.Contains(line, "[Aquarius]") {
			fsecure.Infected = true
			parts := strings.Split(line, "Infected:")
			fsecure.Results.FSE = strings.TrimSuffix(parts[1], "[Aquarius]")
		}
	}

	return fsecure, nil
}

// Get Anti-Virus scanner version
func getFSecureVersion() (version string, database string) {

	// root@4b01c723f943:/malware# /opt/f-secure/fsav/bin/fsav --version
	// EVALUATION VERSION - FULLY FUNCTIONAL - FREE TO USE FOR 30 DAYS.
	// To purchase license, please check http://www.F-Secure.com/purchase/
	//
	// F-Secure Linux Security version 11.00 build 79
	//
	// F-Secure Anti-Virus CLI Command line client version:
	// 	F-Secure Anti-Virus CLI version 1.0  build 0060
	//
	// F-Secure Anti-Virus CLI Daemon version:
	// 	F-Secure Anti-Virus Daemon version 1.0  build 0117
	//
	// Database version: 2016-09-19_01
	//
	// Scanner Engine versions:
	// 	F-Secure Corporation Hydra engine version 5.15 build 154
	// 	F-Secure Corporation Hydra database version 2016-09-16_01
	//
	// 	F-Secure Corporation Aquarius engine version 1.0 build 3
	// 	F-Secure Corporation Aquarius database version 2016-09-19_01
	//
	// Portions:
	// Copyright (c) 1994-2010 Lua.org, PUC-Rio.
	// Copyright (c) Reuben Thomas 2000-2010.
	//
	// For full license information on Hydra engine please see licenses-fselinux.txt in the databases folder

	exec.Command("/opt/f-secure/fsav/bin/fsavd").Output()
	versionOut := utils.RunCommand("/opt/f-secure/fsav/bin/fsav", "--version")

	lines := strings.Split(versionOut, "\n")

	for _, line := range lines {

		if strings.Contains(line, "F-Secure Linux Security version") {
			version = strings.TrimSpace(strings.TrimPrefix(line, "F-Secure Linux Security version"))
		}

		if strings.Contains(line, "Database version:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				database = strings.TrimSpace(parts[1])
				break
			} else {
				log.Error("Umm... ", parts)
			}
		}

	}

	return
}

func parseUpdatedDate(date string) string {
	layout := "Mon, 02 Jan 2006 15:04:05 +0000"
	t, _ := time.Parse(layout, date)
	return fmt.Sprintf("%d%02d%02d", t.Year(), t.Month(), t.Day())
}

func getUpdatedDate() string {
	if _, err := os.Stat("/opt/malice/UPDATED"); os.IsNotExist(err) {
		return BuildTime
	}
	updated, err := ioutil.ReadFile("/opt/malice/UPDATED")
	utils.Assert(err)
	return string(updated)
}

func printStatus(resp gorequest.Response, body string, errs []error) {
	fmt.Println(resp.Status)
}

func updateAV() error {
	fmt.Println("Updating FSecure...")
	// FSecure needs to have the daemon started first
	exec.Command("/etc/init.d/fsaua", "start").Output()
	exec.Command("/etc/init.d/fsupdate", "start").Output()

	fmt.Println(utils.RunCommand(
		"/opt/f-secure/fsav/bin/dbupdate",
		"/opt/f-secure/fsdbupdate9.run",
	))

	// Update UPDATED file
	t := time.Now().Format("20060102")
	err := ioutil.WriteFile("/opt/malice/UPDATED", []byte(t), 0644)
	return err
}

func printMarkDownTable(fsecure FSecure) {

	fmt.Println("#### F-Secure")
	table := clitable.New([]string{"Infected", "Result", "Engine", "Updated"})
	table.AddRow(map[string]interface{}{
		"Infected": fsecure.Results.Infected,
		"Results":  fsecure.Results.Results.Aquarius,
		"Engine":   fsecure.Results.Engine,
		"Updated":  fsecure.Results.Updated,
	})
	table.Markdown = true
	table.Print()
}

// writeToDatabase upserts plugin results into Database
func writeToDatabase(results pluginResults) {
	// connect to RethinkDB
	session, err := r.Connect(r.ConnectOpts{
		Address:  fmt.Sprintf("%s:28015", utils.Getopt("MALICE_RETHINKDB", "rethink")),
		Timeout:  5 * time.Second,
		Database: "malice",
	})
	if err != nil {
		log.Debug(err)
		return
	}
	defer session.Close()

	res, err := r.Table("samples").Get(results.ID).Run(session)
	utils.Assert(err)
	defer res.Close()

	if res.IsNil() {
		// upsert into RethinkDB
		resp, err := r.Table("samples").Insert(results, r.InsertOpts{Conflict: "replace"}).RunWrite(session)
		utils.Assert(err)
		log.Debug(resp)
	} else {
		resp, err := r.Table("samples").Get(results.ID).Update(map[string]interface{}{
			"plugins": map[string]interface{}{
				category: map[string]interface{}{
					name: results.Data,
				},
			},
		}).RunWrite(session)
		utils.Assert(err)

		log.Debug(resp)
	}
}

var appHelpTemplate = `Usage: {{.Name}} {{if .Flags}}[OPTIONS] {{end}}COMMAND [arg...]

{{.Usage}}

Version: {{.Version}}{{if or .Author .Email}}

Author:{{if .Author}}
  {{.Author}}{{if .Email}} - <{{.Email}}>{{end}}{{else}}
  {{.Email}}{{end}}{{end}}
{{if .Flags}}
Options:
  {{range .Flags}}{{.}}
  {{end}}{{end}}
Commands:
  {{range .Commands}}{{.Name}}{{with .ShortName}}, {{.}}{{end}}{{ "\t" }}{{.Usage}}
  {{end}}
Run '{{.Name}} COMMAND --help' for more information on a command.
`

func main() {
	cli.AppHelpTemplate = appHelpTemplate
	app := cli.NewApp()
	app.Name = "f-secure"
	app.Author = "blacktop"
	app.Email = "https://github.com/blacktop"
	app.Version = Version + ", BuildTime: " + BuildTime
	app.Compiled, _ = time.Parse("20060102", BuildTime)
	app.Usage = "Malice F-Secure AntiVirus Plugin"
	var rethinkdb string
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose, V",
			Usage: "verbose output",
		},
		cli.StringFlag{
			Name:        "rethinkdb",
			Value:       "",
			Usage:       "rethinkdb address for Malice to store results",
			EnvVar:      "MALICE_RETHINKDB",
			Destination: &rethinkdb,
		},
		cli.BoolFlag{
			Name:  "table, t",
			Usage: "output as Markdown table",
		},
		cli.BoolFlag{
			Name:   "post, p",
			Usage:  "POST results to Malice webhook",
			EnvVar: "MALICE_ENDPOINT",
		},
		cli.BoolFlag{
			Name:   "proxy, x",
			Usage:  "proxy settings for Malice webhook endpoint",
			EnvVar: "MALICE_PROXY",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:    "update",
			Aliases: []string{"u"},
			Usage:   "Update virus definitions",
			Action: func(c *cli.Context) error {
				return updateAV()
			},
		},
	}
	app.Action = func(c *cli.Context) error {
		path := c.Args().First()

		if _, err := os.Stat(path); os.IsNotExist(err) {
			utils.Assert(err)
		}
		if c.Bool("verbose") {
			log.SetLevel(log.DebugLevel)
		} else {
			r.Log.Out = ioutil.Discard
		}

		var results ResultsData

		results, err := ParseFSecureOutput(utils.RunCommand("/opt/f-secure/fsav/bin/fsav", "--virus-action1=none", path), path)
		if err != nil {
			// If fails try a second time
			results, err = ParseFSecureOutput(utils.RunCommand("/opt/f-secure/fsav/bin/fsav", "--virus-action1=none", path), path)
			utils.Assert(err)
		}

		// upsert into Database
		writeToDatabase(pluginResults{
			ID:   utils.Getopt("MALICE_SCANID", utils.GetSHA256(path)),
			Data: results,
		})

		fsecure := FSecure{
			Results: results,
		}

		if c.Bool("table") {
			printMarkDownTable(fsecure)
		} else {
			fsecureJSON, err := json.Marshal(fsecure)
			utils.Assert(err)
			if c.Bool("post") {
				request := gorequest.New()
				if c.Bool("proxy") {
					request = gorequest.New().Proxy(os.Getenv("MALICE_PROXY"))
				}
				request.Post(os.Getenv("MALICE_ENDPOINT")).
					Set("Task", path).
					Send(fsecureJSON).
					End(printStatus)
			}
			fmt.Println(string(fsecureJSON))
		}
		return nil
	}

	err := app.Run(os.Args)
	utils.Assert(err)
}
