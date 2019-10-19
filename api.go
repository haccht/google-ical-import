package main

import (
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	calendar "google.golang.org/api/calendar/v3"
)

func getCalendarService() (*calendar.Service, error) {
	client, err := newOAuthClient()
	if err != nil {
		return nil, fmt.Errorf("Unable to create api client: %v", err)
	}

	service, err := calendar.New(client)
	if err != nil {
		return nil, fmt.Errorf("Unable to create calendar service: %v", err)
	}

	return service, nil
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

		err = saveToken(cachePath, token)
		if err != nil {
			return nil, err
		}
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

func saveToken(file string, token *oauth2.Token) error {
	f, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("Warning: failed to cache oauth token: %v", err)
	}
	defer f.Close()

	gob.NewEncoder(f).Encode(token)
	return nil
}
