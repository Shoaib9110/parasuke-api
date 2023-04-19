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
		e.Router.GET("/api/parasuke/user",
			func(c echo.Context) error {
				jwtToken := c.Request().Header["Authorization"]
				if len(jwtToken) == 0 || jwtToken[0] == "" {
					return apis.NewNotFoundError("JWT_TOKEN is required", "JWT_TOKEN not found")
				}

				user, err := app.Dao().FindAuthRecordByToken(jwtToken[0], app.Settings().RecordAuthToken.Secret)
				if err != nil {
					return apis.NewNotFoundError("The user record does not exist.", err)
				}

				now := time.Now()
				expiry := now.AddDate(0, 0, 15)

				token := &Oauth2Token{
					AccessToken:  user.GetString("googleAccessToken"),
					TokenType:    "Bearer",
					RefreshToken: user.GetString("googleRefreshToken"),
					Expiry:       expiry,
				}

				b, err := os.ReadFile("credentials.json")
				if err != nil {
					return apis.NewNotFoundError("Unable to read client secret file", err)

				}

				config, err := google.ConfigFromJSON(b, calendar.CalendarScope)
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

				}

				return c.JSON(http.StatusOK, events)
			},
			apis.ActivityLogger(app),
			apis.RequireSameContextRecordAuth(), // TODO: set your required middleware
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
