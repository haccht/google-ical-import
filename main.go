package main

import (
	"context"
	"encoding/gob"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
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

	client, err := newOAuthClient()
	if err != nil {
		log.Fatalf("Unable to create api client: %v", err)
	}

	c, err := calendar.New(client)
	if err != nil {
		log.Fatalf("Unable to create calendar service: %v", err)
	}

	id, err := getCalendarID(c)
	if err != nil {
		log.Fatalf("Unable to get calendar: %v", err)
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

func getCalendarID(c *calendar.Service) (string, error) {
	if *calendarId != "" {
		return *calendarId, nil
	}

	calendars, err := c.CalendarList.List().Fields("items/id").Do()
	if err != nil {
		return "", fmt.Errorf("Unable to retrieve list of calendars: %v", err)
	}

	for i, v := range calendars.Items {
		fmt.Printf("%d:  %v\n", i, v.Id)
	}
	fmt.Printf("Calendar to import: ")

	var index int
	if _, err := fmt.Scan(&index); err != nil {
		return "", fmt.Errorf("Invalid input: %v", err)
	}

	return calendars.Items[index].Id, nil
}

func newOAuthClient() (*http.Client, error) {
	cwd, _ := os.Getwd()

	clientSecret, err := ioutil.ReadFile(filepath.Join(cwd, secretFile))
	if err != nil {
		return nil, fmt.Errorf("Unable to read client secret file: %v", err)
	}

	config, err := google.ConfigFromJSON([]byte(clientSecret), calendar.CalendarScope)
	if err != nil {
		return nil, fmt.Errorf("Unable to parse client secret file into config: %v", err)
	}
	cachePath := filepath.Join(cwd, "credentials")

	token, err := tokenFromFile(cachePath)
	if err != nil {
		token, err = tokenFromWeb(config)
		if err != nil {
			return nil, err
		}

		saveToken(cachePath, token)
	}

	ctx := context.Background()
	return config.Client(ctx, token), nil
}

func tokenFromFile(file string) (*oauth2.Token, error) {
	if !*cacheToken {
		return nil, errors.New("--cachetoken is false")
	}

	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}

	t := new(oauth2.Token)
	err = gob.NewDecoder(f).Decode(t)
	return t, err
}

func tokenFromWeb(config *oauth2.Config) (*oauth2.Token, error) {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Retrieve your authorization code using: %v\nCode: ", authURL)

	var code string
	if _, err := fmt.Scan(&code); err != nil {
		return nil, fmt.Errorf("Unable to read authorization code: %v", err)
	}

	token, err := config.Exchange(oauth2.NoContext, code)
	if err != nil {
		return nil, fmt.Errorf("Unable to retrieve token from web: %v", err)
	}

	return token, nil
}

func saveToken(file string, token *oauth2.Token) {
	f, err := os.Create(file)
	if err != nil {
		log.Printf("Warning: failed to cache oauth token: %v", err)
		return
	}
	defer f.Close()

	gob.NewEncoder(f).Encode(token)
}
