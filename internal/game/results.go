package game

import (
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"time"

	"weezel/jukeboxjury/internal/logger"
)

//go:embed assets/results_template.html
var resultsTemplate string

var timeZone = func() *time.Location {
	loc, err := time.LoadLocation("Europe/Helsinki")
	if err != nil {
		logger.Logger.Fatal().Err(err).Msg("Time zone not found")
	}
	return loc
}()

var funcMap = template.FuncMap{
	"timeNow": func() string {
		return time.Now().In(timeZone).Format("2006-01-02T15:04:05")
	},
}

var tmpl = template.Must(template.New("results").Funcs(funcMap).Parse(resultsTemplate))

func renderResults(results []*Panelist, output io.Writer) error {
	if err := tmpl.Execute(output, results); err != nil {
		return fmt.Errorf("rendering template: %w", err)
	}

	return nil
}
