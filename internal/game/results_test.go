package game

import (
	_ "embed"
	"os"
	"testing"
)

func Test_renderResults(t *testing.T) {
	panelists := []*Panelist{
		{
			Name: "Satan",
			ReceivedReviews: []*Review{
				{From: "Jesus", Rating: 10, Review: "Great song1"},
			},
			Song: &Song{
				AverageScore: 10,
				Description:  "My favourite song",
				URL:          "https://example.com/satan_you_rock",
			},
		},
		{
			Name: "Jesus",
			ReceivedReviews: []*Review{
				{From: "Satan", Rating: 1, Review: "Terrible song1"},
			},
			Song: &Song{
				AverageScore: 1,
				Description:  "Hallelujah 🤘",
				URL:          "https://example.com/hesus",
			},
		},
	}

	tests := []struct {
		name    string
		wantErr bool
	}{
		{
			name:    "",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := renderResults(panelists, os.Stdout); (err != nil) != tt.wantErr {
				t.Errorf("renderResults() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
