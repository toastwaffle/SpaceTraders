package app

import (
	"fmt"
	"regexp"
	"strings"

	"fivebit.co.uk/spacetraders/prompt"
)

var (
	sectorRegexp = regexp.MustCompile(`[A-Z]\d`)
	systemRegexp = regexp.MustCompile(`[A-Z]\d-[A-Z]{2}\d{2}`)
	waypointRegexp = regexp.MustCompile(`[A-Z]\d-[A-Z]{2}\d{2}-\d{5}[A-Z]`)
)

type Sector string

func parseSector(s string) (Sector, error) {
	s = strings.TrimSpace(s)
	if !sectorRegexp.MatchString(s) {
		return Sector(""), fmt.Errorf("Expected sector of form X1; got %q", s)
	}
	return Sector(s), nil
}

func (s Sector) String() string {
	return string(s)
}

type System struct {
	sector Sector
	system string
}

func parseSystem(s string) (System, error) {
	s = strings.TrimSpace(s)
	if !systemRegexp.MatchString(s) {
		return System{}, fmt.Errorf("Expected sector of form X1-DF55; got %q", s)
	}
	parts := strings.Split(s, "-")
	return System{
		sector: Sector(parts[0]),
		system: parts[1],
	}, nil
}

func (s System) String() string {
	return fmt.Sprintf("%s-%s", s.sector, s.system)
}

type Waypoint struct {
	system System
	waypoint string
}

func (w Waypoint) String() string {
	return fmt.Sprintf("%s-%s", w.system, w.waypoint)
}

func ParseWaypoint(wp string) (Waypoint, error) {
	wp = strings.TrimSpace(wp)
	if !waypointRegexp.MatchString(wp) {
		return Waypoint{}, fmt.Errorf("Expected waypoint of form X1-DF55-20250Z; got %q", wp)
	}
	parts := strings.Split(wp, "-")
	return Waypoint{
		system: System{
			sector: Sector(parts[0]),
			system: parts[1],
		},
		waypoint: parts[2],
	}, nil
}

func ReadWaypoint() (Waypoint, error) {
	wpStr, err := prompt.Prompt("Enter waypoint", func(input string) error {
		_, err := ParseWaypoint(input)
		return err
	})
	if err != nil {
		return Waypoint{}, err
	}

	return ParseWaypoint(wpStr)
}
