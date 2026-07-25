package imdbapi

import "testing"

// imdbapi.dev sometimes returns partial episode release dates (month/day/year 0).
// mapEpisode must keep only complete dates and drop partial ones.
func TestMapEpisode_DropsPartialReleaseDate(t *testing.T) {
	full := mapEpisode(wireEpisode{ID: "tt1", ReleaseDate: &wireReleaseDate{Year: 2008, Month: 1, Day: 20}})
	if full.ReleaseDate == nil || full.ReleaseDate.Year != 2008 || full.ReleaseDate.Month != 1 || full.ReleaseDate.Day != 20 {
		t.Fatalf("complete date should be kept, got %+v", full.ReleaseDate)
	}

	partial := []*wireReleaseDate{
		{Year: 2008, Month: 0, Day: 20}, // month 0
		{Year: 2008, Month: 1, Day: 0},  // day 0
		{Year: 0, Month: 1, Day: 20},    // year 0
	}
	for _, rd := range partial {
		if got := mapEpisode(wireEpisode{ID: "tt1", ReleaseDate: rd}); got.ReleaseDate != nil {
			t.Fatalf("partial date %+v should be dropped, got %+v", rd, got.ReleaseDate)
		}
	}
}
