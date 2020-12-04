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

func findChangedFilesUsingCommand(cwd string, args ...string) []string {
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

func findChangedFiles(cwd string, option *Option) []string {
	if option == nil || (!option.WithAncestor && !option.LastCommit && option.ChangedSince == "") {
		var wg sync.WaitGroup
		wg.Add(2)
		res := make([][]string, 2)
		go func() {
			res[0] = findChangedFilesUsingCommand(cwd, "diff", "--cached", "--name-only")
			wg.Done()
		}()
		go func() {
			res[1] = findChangedFilesUsingCommand(cwd, "ls-files", "--other", "--modified", "--exclude-standard")
			wg.Done()
		}()
		wg.Wait()
		return uniqueCombineOutputs(res...)
	}

	if option.LastCommit {
		return findChangedFilesUsingCommand(cwd, "show", "--name-only", "--pretty=format:", "HEAD")
	}

	changedSince := option.ChangedSince
	if option.WithAncestor {
		changedSince = "HEAD^"
	}

	var wg sync.WaitGroup
	wg.Add(3)
	res := make([][]string, 3)

	go func() {
		res[0] = findChangedFilesUsingCommand(cwd, "diff", "--name-only", fmt.Sprintf("%s...HEAD", changedSince))
		wg.Done()
	}()

	go func() {
		res[1] = findChangedFilesUsingCommand(cwd, "diff", "--cached", "--name-only")
		wg.Done()
	}()

	go func() {
		res[2] = findChangedFilesUsingCommand(cwd, "ls-files", "--other", "--modified", "--exclude-standard")
		wg.Done()
	}()

	wg.Wait()

	return uniqueCombineOutputs(res...)
}

func filter(result []string, reg *regexp.Regexp) []string {
	res := make([]string, 0)
	for _, s := range result {
		if reg.MatchString(s) {
			res = append(res, s)
		}
	}
	return res
}

func main() {
	option := &Option{}
	app := kingpin.New("changed-files", "go port jest-changed-files.")
	app.Flag("lastCommit", "If since lastCommit.").Short('l').BoolVar(&option.LastCommit)
	app.Flag("withAncestor", "If with ancestor.").Short('w').BoolVar(&option.WithAncestor)
	app.Flag("changedSince", "Get changed since commit.").Short('s').StringVar(&option.ChangedSince)
	filterReg := app.Flag("filter", "Filter regex.").Short('f').String()
	command := app.Arg("command", "Command prefix.").String()
	folder := app.Flag("folder", "If return folder path.").Bool()

	app.Version(buildVersion(version, commit, date, builtBy))
	app.VersionFlag.Short('v')
	app.HelpFlag.Short('h')

	kingpin.MustParse(app.Parse(os.Args[1:]))

	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	files := findChangedFiles(cwd, option)

	if *filterReg != "" {
		reg := regexp.MustCompile(*filterReg)
		files = filter(files, reg)
	}

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
