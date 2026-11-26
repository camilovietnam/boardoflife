package books

import (
	"errors"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func TestValidateNormalizes(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   Input
		want Draft
	}{
		{
			"trims text",
			Input{Name: "  Dune ", Author: "\tFrank Herbert\n", Status: StatusReading, Progress: ptr(42.0)},
			Draft{Name: "Dune", Author: "Frank Herbert", Status: StatusReading, Progress: 42},
		},
		{
			"to_read forces progress to 0",
			Input{Name: "a", Author: "b", Status: StatusToRead, Progress: ptr(70.0)},
			Draft{Name: "a", Author: "b", Status: StatusToRead, Progress: 0},
		},
		{
			"finished forces progress to 100",
			Input{Name: "a", Author: "b", Status: StatusFinished},
			Draft{Name: "a", Author: "b", Status: StatusFinished, Progress: 100},
		},
		{
			"progress defaults to 0 when reading",
			Input{Name: "a", Author: "b", Status: StatusReading},
			Draft{Name: "a", Author: "b", Status: StatusReading},
		},
		{
			"half-star rating",
			Input{Name: "a", Author: "b", Status: StatusReading, Rating: ptr(3.5)},
			Draft{Name: "a", Author: "b", Status: StatusReading, Rating: ptr(3.5)},
		},
		{
			"max length counts characters, not bytes",
			Input{Name: strings.Repeat("é", 300), Author: "b", Status: StatusReading},
			Draft{Name: strings.Repeat("é", 300), Author: "b", Status: StatusReading},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.in.Validate()
			if err != nil {
				t.Fatal(err)
			}
			if got.Name != tc.want.Name || got.Author != tc.want.Author ||
				got.Status != tc.want.Status || got.Progress != tc.want.Progress {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
			if (got.Rating == nil) != (tc.want.Rating == nil) ||
				got.Rating != nil && *got.Rating != *tc.want.Rating {
				t.Fatalf("rating = %v, want %v", got.Rating, tc.want.Rating)
			}
		})
	}
}

func TestValidateRejects(t *testing.T) {
	ok := Input{Name: "a", Author: "b", Status: StatusReading}
	with := func(f func(*Input)) Input { in := ok; f(&in); return in }

	for _, tc := range []struct {
		name  string
		in    Input
		field string
	}{
		{"blank name", with(func(i *Input) { i.Name = "  \t" }), "name"},
		{"long name", with(func(i *Input) { i.Name = strings.Repeat("x", 301) }), "name"},
		{"blank author", with(func(i *Input) { i.Author = "" }), "author"},
		{"long author", with(func(i *Input) { i.Author = strings.Repeat("x", 301) }), "author"},
		{"empty status", with(func(i *Input) { i.Status = "" }), "status"},
		{"unknown status", with(func(i *Input) { i.Status = "done" }), "status"},
		{"negative progress", with(func(i *Input) { i.Progress = ptr(-1.0) }), "progress"},
		{"progress over 100", with(func(i *Input) { i.Progress = ptr(101.0) }), "progress"},
		{"fractional progress", with(func(i *Input) { i.Progress = ptr(10.5) }), "progress"},
		{"rating below 1", with(func(i *Input) { i.Rating = ptr(0.5) }), "rating"},
		{"rating zero", with(func(i *Input) { i.Rating = ptr(0.0) }), "rating"},
		{"rating above 5", with(func(i *Input) { i.Rating = ptr(5.5) }), "rating"},
		{"rating off-step", with(func(i *Input) { i.Rating = ptr(4.3) }), "rating"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.in.Validate()
			var fields FieldErrors
			if !errors.As(err, &fields) {
				t.Fatalf("err = %v, want FieldErrors", err)
			}
			if fields[tc.field] == "" {
				t.Fatalf("no error for %q: %v", tc.field, fields)
			}
		})
	}
}

func TestValidateReportsEveryField(t *testing.T) {
	_, err := Input{Status: "x", Progress: ptr(500.0), Rating: ptr(9.0)}.Validate()
	var fields FieldErrors
	if !errors.As(err, &fields) || len(fields) != 5 {
		t.Fatalf("fields = %v", fields)
	}
}

func TestLikeEscaper(t *testing.T) {
	if got, want := likeEscaper.Replace(`100%_\x`), `100\%\_\\x`; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestHalfStars(t *testing.T) {
	if halfStars(nil).Valid {
		t.Fatal("nil rating must be NULL")
	}
	for in, want := range map[float64]int16{1: 2, 1.5: 3, 3.5: 7, 5: 10} {
		if got := halfStars(&in); !got.Valid || got.Int16 != want {
			t.Errorf("halfStars(%v) = %v, want %d", in, got, want)
		}
	}
}

func TestValidateOptionalMetadata(t *testing.T) {
	base := Input{Name: "a", Author: "b", Status: StatusReading}
	with := func(f func(*Input)) Input { in := base; f(&in); return in }

	t.Run("normalizes", func(t *testing.T) {
		got, err := with(func(i *Input) {
			i.CoverURL = ptr("  HTTPS://Example.com/c.jpg?x=1 ")
			i.Description = ptr("  hello \n")
			i.ExternalID = ptr(" vol-1 ")
		}).Validate()
		if err != nil {
			t.Fatal(err)
		}
		if *got.CoverURL != "HTTPS://Example.com/c.jpg?x=1" || *got.Description != "hello" || *got.ExternalID != "vol-1" {
			t.Fatalf("got %q %q %q", *got.CoverURL, *got.Description, *got.ExternalID)
		}
	})

	t.Run("blank becomes nil", func(t *testing.T) {
		got, err := with(func(i *Input) {
			i.CoverURL, i.Description, i.ExternalID = ptr(""), ptr("  \t"), ptr(" ")
		}).Validate()
		if err != nil {
			t.Fatal(err)
		}
		if got.CoverURL != nil || got.Description != nil || got.ExternalID != nil {
			t.Fatalf("got %+v", got)
		}
	})

	t.Run("limits count characters", func(t *testing.T) {
		_, err := with(func(i *Input) {
			i.Description = ptr(strings.Repeat("é", 5000))
			i.ExternalID = ptr(strings.Repeat("é", 100))
			i.CoverURL = ptr("https://e.com/" + strings.Repeat("a", 2000-len("https://e.com/")))
		}).Validate()
		if err != nil {
			t.Fatal(err)
		}
	})

	for _, tc := range []struct {
		name  string
		in    Input
		field string
	}{
		{"http cover", with(func(i *Input) { i.CoverURL = ptr("http://e.com/c.jpg") }), "cover_url"},
		{"javascript cover", with(func(i *Input) { i.CoverURL = ptr("javascript:alert(1)") }), "cover_url"},
		{"data cover", with(func(i *Input) { i.CoverURL = ptr("data:image/png;base64,AAAA") }), "cover_url"},
		{"relative cover", with(func(i *Input) { i.CoverURL = ptr("/c.jpg") }), "cover_url"},
		{"scheme-relative cover", with(func(i *Input) { i.CoverURL = ptr("//e.com/c.jpg") }), "cover_url"},
		{"hostless cover", with(func(i *Input) { i.CoverURL = ptr("https:///c.jpg") }), "cover_url"},
		{"cover with spaces", with(func(i *Input) { i.CoverURL = ptr("https://e .com/c.jpg") }), "cover_url"},
		{"long cover", with(func(i *Input) { i.CoverURL = ptr("https://e.com/" + strings.Repeat("a", 2000)) }), "cover_url"},
		{"long description", with(func(i *Input) { i.Description = ptr(strings.Repeat("x", 5001)) }), "description"},
		{"long external id", with(func(i *Input) { i.ExternalID = ptr(strings.Repeat("x", 101)) }), "external_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.in.Validate()
			var fields FieldErrors
			if !errors.As(err, &fields) || fields[tc.field] == "" {
				t.Fatalf("err = %v, want error for %q", err, tc.field)
			}
		})
	}
}
