package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/apis"
	"github.com/pocketbase/pocketbase/core"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

type Oauth2Token struct {
	AccessToken  string
	TokenType    string
	RefreshToken string
	Expiry       time.Time
}

func main() {
	app := pocketbase.New()

	app.OnBeforeServe().Add(func(e *core.ServeEvent) error {
		e.Router.GET("/api/parasuke/user/:id", // TODO: change path name as expected
			func(c echo.Context) error {
				id := c.PathParam("id")
				record, err := app.Dao().FindRecordById("users", id) // TODO: change table name as expected
				if err != nil {
					return apis.NewNotFoundError("The article does not exist.", err)
				}

				expiry, err := time.Parse("2006-01-02", record.GetString("Expiry"))
				if err != nil {
					return apis.NewNotFoundError("unable to Parse token Expiry Date", err)
				}

				token := &Oauth2Token{
					AccessToken:  record.GetString("AccessToken"),
					TokenType:    "Bearer",
					RefreshToken: record.GetString("RefreshToken"),
					Expiry:       expiry,
				}

				b, err := os.ReadFile("credentials.json")
				if err != nil {
					return apis.NewNotFoundError("Unable to read client secret file", err)

				}

				config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
				if err != nil {
					return apis.NewNotFoundError("Unable to parse client secret file to config", err)
				}

				client := getClient(config, token)

				ctx := context.Background()
				srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
				if err != nil {
					return apis.NewNotFoundError("Unable to retrieve Calendar client", err)
				}

				t := time.Now().Format(time.RFC3339)
				events, err := srv.Events.List("primary").ShowDeleted(false).
					SingleEvents(true).TimeMin(t).MaxResults(10).OrderBy("startTime").Do()
				if err != nil {
					return apis.NewNotFoundError("Unable to retrieve next ten of the user's events", err)
				}

				if len(events.Items) == 0 {
					fmt.Println("No upcoming events found.")
					return apis.NewNotFoundError("No upcoming events found.", err)

				} else {
					// TODO: events.Items in record
					for _, item := range events.Items {
						date := item.Start.DateTime
						if date == "" {
							date = item.Start.Date
						}
						fmt.Printf("%v (%v)\n", item.Summary, date)
					}
				}

				// TODO:  Return events Items field as expected using this
				apis.EnrichRecord(c, app.Dao(), record)

				return c.JSON(http.StatusOK, record)
			},
			apis.ActivityLogger(app),
		)

		return nil
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}

func getClient(config *oauth2.Config, token *Oauth2Token) *http.Client {

	tok := &oauth2.Token{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		Expiry:       token.Expiry,
	}
	return config.Client(context.Background(), tok)
}
