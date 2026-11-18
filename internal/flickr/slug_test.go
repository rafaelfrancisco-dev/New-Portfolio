package flickr

import "testing"

func TestSlugify(t *testing.T) {
	for in, want := range map[string]string{
		"Wildlife":                "wildlife",
		"Aves de Portugal":        "aves-de-portugal",
		"Serra da Lousã, Inverno": "serra-da-lousa-inverno",
		"  --Árvores & Rios--  ":  "arvores-rios",
		"★":                       "",
	} {
		if got := Slugify(in); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAssignSlugsResolvesCollisions(t *testing.T) {
	albums := []Album{{ID: "1", Title: "Birds"}, {ID: "2", Title: "birds"}, {ID: "3", Title: "★"}, {ID: "4", Title: "Mondego"}}
	assignSlugs(albums)
	want := []string{"birds-1", "birds-2", "3", "mondego"}
	for i, a := range albums {
		if a.Slug != want[i] {
			t.Errorf("album %s slug = %q, want %q", a.ID, a.Slug, want[i])
		}
	}
}
