package importer_test

import (
	"testing"

	"github.com/stevejefferson/trac2gitea/importer"
)

func TestMapRevisionsNil(t *testing.T) {
	var revisionMap map[string]string = nil
	input := "foo"
	expected := "foo"
	result := importer.MapRevisions(input, revisionMap)

	if result != expected {
		t.Errorf("Expected %s but got %s", expected, result)
	}
}

func TestMapBasic(t *testing.T) {
	revisionMap := map[string]string{
		"r1234": "deadf00d",
	}
	input := "Fixed in r1234"
	expected := "Fixed in deadf00d"

	result := importer.MapRevisions(input, revisionMap)

	if result != expected {
		t.Errorf("Expected '%s' but got '%s'", expected, result)
	}
}

func TestMapBasicAtStart(t *testing.T) {
	revisionMap := map[string]string{
		"r1234": "deadf00d",
	}
	input := "r1234 fixes the problem"
	expected := "deadf00d fixes the problem"

	result := importer.MapRevisions(input, revisionMap)

	if result != expected {
		t.Errorf("Expected '%s' but got '%s'", expected, result)
	}
}

func TestMapNotFound(t *testing.T) {
	revisionMap := map[string]string{
		"r1234": "deadf00d",
	}
	input := "Fixed in r9999"
	expected := "Fixed in r9999"

	result := importer.MapRevisions(input, revisionMap)

	if result != expected {
		t.Errorf("Expected '%s' but got '%s'", expected, result)
	}
}

func TestMapRange(t *testing.T) {
	revisionMap := map[string]string{
		"r1234": "deadf00d",
		"r4567": "badcafe",
	}
	input := "Implemented in r1234-r4567"
	expected := "Implemented in deadf00d..badcafe"

	result := importer.MapRevisions(input, revisionMap)

	if result != expected {
		t.Errorf("Expected '%s' but got '%s'", expected, result)
	}
}

func TestMapCommitReference(t *testing.T) {
	revisionMap := map[string]string{
		"r4485": "deadf00d",
	}
	input := `In [4485]changeset:"4485":`
	expected := `See deadf00d`

	result := importer.MapRevisions(input, revisionMap)

	if result != expected {
		t.Errorf("Expected '%s' but got '%s'", expected, result)
	}
}

func TestMapOldStyleCommitReference(t *testing.T) {
	revisionMap := map[string]string{
		"r4485": "deadf00d",
	}
	input := `In changeset:"4485":`
	expected := `See deadf00d`

	result := importer.MapRevisions(input, revisionMap)

	if result != expected {
		t.Errorf("Expected '%s' but got '%s'", expected, result)
	}
}

func TestMapOlderStyleCommitReference(t *testing.T) {
	revisionMap := map[string]string{
		"r4485": "deadf00d",
	}
	input := `In changeset:4485:`
	expected := `See deadf00d`

	result := importer.MapRevisions(input, revisionMap)

	if result != expected {
		t.Errorf("Expected '%s' but got '%s'", expected, result)
	}
}

func TestMapMergeRange(t *testing.T) {
	revisionMap := map[string]string{
		"r4415": "deadf00d",
		"r4419": "badcafe",
	}
	input := "Merged revision(s) r4415-4419 from branches/multi_monitor:"
	expected := "Merged revision(s) deadf00d..badcafe from branches/multi_monitor:"

	result := importer.MapRevisions(input, revisionMap)

	if result != expected {
		t.Errorf("Expected '%s' but got '%s'", expected, result)
	}
}
