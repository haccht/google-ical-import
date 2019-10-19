package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	calendar "google.golang.org/api/calendar/v3"
)

var (
	timezone   = "Asia/Tokyo"
	secretFile = "client_secret.json"

	calendarId = flag.String("calendar", "", "Calendar ID")
	cacheToken = flag.Bool("cachetoken", true, "Cache the OAuth 2.0 token")
)

func main() {
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] filepath", os.Args[0])
		flag.PrintDefaults()
	}

	icsFile := flag.Arg(0)
	src, err := ParseFile(icsFile)
	if err != nil {
		log.Fatalf("Unable to parse ics file: %v", err)
	}

	c, err := getCalendarService()
	if err != nil {
		log.Fatalf("Unable to get calendar: %v", err)
	}

	id, err := getCalendarID(c)
	if err != nil {
		log.Fatalf("Unable to specify calendar: %v", err)
	}
	fmt.Printf("Import events into Google Calendar: %s\n", id)

	for _, v := range src.Events() {
		ev, err := newEvent(v)
		if err != nil {
			log.Printf("Faild to generate a event: %v", err)
			continue
		}

		if ev.Summary == "" {
			continue
		}

		fmt.Printf("Importing '%s'...\n", ev.Summary)
		_, err = c.Events.Import(id, ev).Do()
		if err != nil {
			log.Printf("Failed to import the event: %v", err)
			continue
		}
	}
}

func newEvent(e *Event) (*calendar.Event, error) {
	ev := new(calendar.Event)
	tz, _ := time.LoadLocation(timezone)

	for _, prop := range e.Properties {
		switch prop.Name {
		case "UID":
			ev.ICalUID = prop.Value
		case "SUMMARY":
			ev.Summary = prop.Value
		case "LOCATION":
			ev.Location = prop.Value
		case "DESCRIPTION":
			descr := strings.Replace(prop.Value, "\\n", "\n", -1)
			ev.Description = descr
		case "SEQUENCE":
			seq, _ := strconv.ParseInt(prop.Value, 10, 64)
			ev.Sequence = seq
		case "RRULE", "EXRULE", "RDATE", "EXDATE":
			ev.Recurrence = append(ev.Recurrence, prop.String())
		case "DTSTART":
			dt, err := newEventDateTime(prop, tz)
			if err != nil {
				return nil, err
			}
			ev.Start = dt
		case "DTEND":
			dt, err := newEventDateTime(prop, tz)
			if err != nil {
				return nil, err
			}
			ev.End = dt
		}
	}

	return ev, nil
}

func newEventDateTime(prop *Property, tz *time.Location) (*calendar.EventDateTime, error) {
	value := prop.Value

	re1 := regexp.MustCompile(`^[0-9]{8}T[0-9]{6}`)
	if re1.MatchString(value) {
		datetime, err := time.ParseInLocation("20060102T150405", re1.FindString(value), tz)
		if err != nil {
			return nil, err
		}

		return &calendar.EventDateTime{TimeZone: timezone, DateTime: datetime.Format(time.RFC3339)}, nil
	}

	re2 := regexp.MustCompile(`^[0-9]{8}`)
	if re2.MatchString(value) {
		datetime, err := time.ParseInLocation("20060102", re2.FindString(value), tz)
		if err != nil {
			return nil, err
		}

		return &calendar.EventDateTime{TimeZone: timezone, Date: datetime.Format("2006-01-02")}, nil
	}

	return nil, fmt.Errorf("Unsupported datetime format: %v", value)
}
