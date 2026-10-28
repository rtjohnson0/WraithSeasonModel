package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"propsmodel/internal/espn"
	"propsmodel/internal/sgo"
)

// mergeESPN folds ESPN's injury list into the report predict already has. A player ESPN lists as Out
// whom the main report does not is treated as out (a line on him is junk either way, and his
// teammates' and opponents' features should know). A player ESPN lists as day-to-day whom the main
// report has no status for is marked uncertain, which makes his plays CHECK (not staked). It returns
// the players it changed, for the printout.
func mergeESPN(inj []espn.Injury, byName map[string]int, out, statuses map[int]string) (nowOut, nowMaybe []string) {
	for _, i := range inj {
		pid, ok := byName[sgo.NormName(i.Name)]
		if !ok {
			continue
		}
		if _, already := out[pid]; already {
			continue
		}
		switch {
		case i.IsOut():
			out[pid] = "out (ESPN)"
			nowOut = append(nowOut, fmt.Sprintf("%s (%s)", i.Name, i.Team))
		case i.IsGameTime():
			if st, ok := statuses[pid]; !ok || st == "" {
				statuses[pid] = "Day-To-Day (ESPN)"
				nowMaybe = append(nowMaybe, fmt.Sprintf("%s (%s)", i.Name, i.Team))
			}
		}
	}
	sort.Strings(nowOut)
	sort.Strings(nowMaybe)
	return
}

// loadESPN reads the day's saved ESPN list (written by `pull snapshot` or `pull espn`); nil if absent.
func loadESPN(snapDir, date string) []espn.Injury {
	body, err := os.ReadFile(filepath.Join(snapDir, fmt.Sprintf("espn_injuries_%s.json", date)))
	if err != nil {
		return nil
	}
	inj, err := espn.Parse(body)
	if err != nil {
		fmt.Println("note: could not read the ESPN injury list:", err)
		return nil
	}
	return inj
}

func printESPN(nowOut, nowMaybe []string) {
	if len(nowOut)+len(nowMaybe) == 0 {
		return
	}
	fmt.Println("ESPN INJURY CHECK (players the main injury report does not list):")
	if len(nowOut) > 0 {
		fmt.Printf("  now treated as out: %s\n", strings.Join(nowOut, ", "))
	}
	if len(nowMaybe) > 0 {
		fmt.Printf("  now marked day-to-day (their plays become CHECK): %s\n", strings.Join(nowMaybe, ", "))
	}
	fmt.Println()
}
