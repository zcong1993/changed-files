package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	version = "master"
	commit  = ""
	date    = ""
	builtBy = ""
)

type Option struct {
	LastCommit   bool
	WithAncestor bool
	ChangedSince string
}

func findChangedAndFilter(cwd string, args []string, reg *regexp.Regexp) []string {
	cmd := exec.Command("git", args...)
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		log.Fatal(err)
	}
	s := string(out)
	tmpArr := strings.Split(s, "\n")
	res := make([]string, 0)
	for _, ss := range tmpArr {
		if ss != "" {
			if reg != nil && !reg.MatchString(ss) {
				continue
			}
			res = append(res, filepath.Join(cwd, ss))
		}
	}
	return res
}

func uniqueCombineOutputs(results ...[]string) []string {
	mp := make(map[string]struct{}, 0)
	for _, r := range results {
		for _, s := range r {
			mp[s] = struct{}{}
		}
	}
	res := make([]string, len(mp))
	i := 0
	for k := range mp {
		res[i] = k
		i++
	}
	return res
}

func findChangedFiles(cwd string, option *Option, reg *regexp.Regexp) []string {
	if option == nil || (!option.WithAncestor && !option.LastCommit && option.ChangedSince == "") {
		var wg sync.WaitGroup
		wg.Add(2)
		res := make([][]string, 2)
		go func() {
			res[0] = findChangedAndFilter(cwd, []string{"diff", "--cached", "--name-only"}, reg)
			wg.Done()
		}()
		go func() {
			res[1] = findChangedAndFilter(cwd, []string{"ls-files", "--other", "--modified", "--exclude-standard"}, reg)
			wg.Done()
		}()
		wg.Wait()
		return uniqueCombineOutputs(res...)
	}

	if option.LastCommit {
		return findChangedAndFilter(cwd, []string{"show", "--name-only", "--pretty=format:", "HEAD"}, reg)
	}

	changedSince := option.ChangedSince
	if option.WithAncestor {
		changedSince = "HEAD^"
	}

	var wg sync.WaitGroup
	wg.Add(3)
	res := make([][]string, 3)

	go func() {
		res[0] = findChangedAndFilter(cwd, []string{"diff", "--name-only", fmt.Sprintf("%s...HEAD", changedSince)}, reg)
		wg.Done()
	}()

	go func() {
		res[1] = findChangedAndFilter(cwd, []string{"diff", "--cached", "--name-only"}, reg)
		wg.Done()
	}()

	go func() {
		res[2] = findChangedAndFilter(cwd, []string{"ls-files", "--other", "--modified", "--exclude-standard"}, reg)
		wg.Done()
	}()

	wg.Wait()

	return uniqueCombineOutputs(res...)
}

func main() {
	option := &Option{}
	app := kingpin.New("changed-files", "go port jest-changed-files.")
	app.Flag("lastCommit", "If since lastCommit.").Short('l').BoolVar(&option.LastCommit)
	app.Flag("withAncestor", "If with ancestor.").Short('w').BoolVar(&option.WithAncestor)
	app.Flag("changedSince", "Get changed since commit.").Short('s').StringVar(&option.ChangedSince)
	filterReg := app.Flag("filter", "Filter regex.").Short('f').String()
	folder := app.Flag("folder", "If return folder path.").Bool()
	command := app.Arg("command", "Command prefix.").String()

	app.Version(buildVersion(version, commit, date, builtBy))
	app.VersionFlag.Short('v')
	app.HelpFlag.Short('h')

	kingpin.MustParse(app.Parse(os.Args[1:]))

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	var reg *regexp.Regexp
	if *filterReg != "" {
		reg = regexp.MustCompile(*filterReg)
	}

	files := findChangedFiles(cwd, option, reg)

	if len(files) == 0 {
		os.Exit(1)
	}

	if *folder {
		folders := make([]string, 0)
		for _, fp := range files {
			folders = append(folders, filepath.Dir(fp))
		}
		files = uniqueCombineOutputs(folders)
	}

	fmt.Printf("%s %s", *command, strings.Join(files, " "))
}

func buildVersion(version, commit, date, builtBy string) string {
	var result = version
	if commit != "" {
		result = fmt.Sprintf("%s\ncommit: %s", result, commit)
	}
	if date != "" {
		result = fmt.Sprintf("%s\nbuilt at: %s", result, date)
	}
	if builtBy != "" {
		result = fmt.Sprintf("%s\nbuilt by: %s", result, builtBy)
	}
	return result
}
