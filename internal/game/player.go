package game

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type Song struct {
	Description  string  `json:"description"`
	URL          string  `json:"url"`
	AverageScore float64 `json:"average_score"`
}

func (s Song) String() string {
	return fmt.Sprintf("Description: %s, URL: %s", s.Description, s.URL)
}

type Review struct {
	From   string `json:"from"`
	Review string `json:"review"`
	Rating int    `json:"rating"`
}

type Panelist struct {
	Name            string    `json:"name"`
	Song            *Song     `json:"song"`
	ReceivedReviews []*Review `json:"received_reviews"`
	ReviewGiven     bool
	SongSubmitted   bool
	SongPresented   bool
	uid             int64
}

func NewPanelist(name string, uid int64) *Panelist {
	return &Panelist{
		Name:            name,
		uid:             uid,
		Song:            &Song{},
		ReceivedReviews: []*Review{},
	}
}

var (
	ErrNoReview     = errors.New("no review")
	ErrParseSongURL = errors.New("song URL ist kaput")
)

func (p *Panelist) AddSong(msg Message) error {
	if len(msg.Text) < 1 {
		return ErrNoReview
	}

	splt := strings.Fields(msg.Text)
	possibleURL := splt[len(splt)-1]
	songURL, err := url.Parse(possibleURL)
	if err != nil {
		return fmt.Errorf("song url %q: %w", possibleURL, ErrParseSongURL)
	}

	description := strings.Join(splt[0:len(splt)-1], " ")

	p.Song = &Song{
		Description: description,
		URL:         songURL.String(),
	}
	p.SongSubmitted = true

	return nil
}

type ReviewError struct {
	Err        error
	ErrForUser string
}

func (r ReviewError) Error() string {
	return fmt.Sprintf("add review: %s", r.Err)
}

func (p *Panelist) AddReview(reviewer *Panelist, review string) error {
	rating, err := parseRating(review)
	if err != nil {
		return ReviewError{
			Err: fmt.Errorf("parse rating from user=%s, review=%s, error: %w", reviewer.Name, review, err),
			ErrForUser: "Did you forgot to give the points? " +
				"Those should be in 10/10 format and as a last item.",
		}
	}
	cleanedReview := strings.LastIndex(review, " ")
	if cleanedReview == -1 {
		return ReviewError{
			Err:        fmt.Errorf("parse last space char from user %s, review %q", reviewer.Name, review),
			ErrForUser: "Check that the scoring is last item and separated with a space: ... 5/10",
		}
	}

	p.ReceivedReviews = append(p.ReceivedReviews, &Review{
		Rating: rating,
		From:   reviewer.Name,
		Review: review[0:cleanedReview],
	})

	reviewer.ReviewGiven = true

	return nil
}

var ratingPat = regexp.MustCompile("[0-9]+/[0-9]+$")

// TODO simplify
func parseRating(review string) (int, error) {
	match := ratingPat.FindString(review)
	if match == "" {
		return -1, errors.New("couldn't parse review points")
	}
	points := strings.Split(match, "/")
	numPoints, _ := strconv.Atoi(points[0])
	return numPoints, nil
}
