package stitch

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

var testRevs = []string{
	// no directories
	// "v3.36.0",

	// no elevated permissions
	// "v3.37.0",

	"v3.38.0",
	"v3.39.0",
	"v3.40.0",
	"v3.41.0",

	// #38599 - Update migration directory names
	"fbe5a819a15277d6908eb2f74e49f6b6570fd2de",
}

func TestStitchFrontendDefinitions(t *testing.T) {
	testStitch(t, "frontend", 1528395943, []int{1657279170})
}

func TestStitchCodeintelDefinitions(t *testing.T) {
	testStitch(t, "codeintel", 1000000029, []int{1000000034})
}

func TestStitchCodeinsightsDefinitions(t *testing.T) {
	testStitch(t, "codeinsights", 1000000020, []int{1656517037, 1656608833})
}

func testStitch(t *testing.T, schemaName string, expectedRoot int, expectedLeaves []int) {
	definitions, err := StitchDefinitions(schemaName, testRevs)
	if err != nil {
		t.Fatal(err)
	}

	var leafIDs []int
	for _, migration := range definitions.Leaves() {
		leafIDs = append(leafIDs, migration.ID)
	}

	if rootID := definitions.Root().ID; rootID != expectedRoot {
		t.Fatalf("unexpected root migration. want=%d have=%d", expectedRoot, rootID)
	}
	if len(leafIDs) != len(expectedLeaves) || cmp.Diff(expectedLeaves, leafIDs) != "" {
		t.Fatalf("unexpected root migration. want=%v have=%v", expectedLeaves, leafIDs)
	}
}
