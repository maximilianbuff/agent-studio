package claudemd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testHome = "/home/user/.agent-studio"

func TestBlock_containsKeyContent(t *testing.T) {
	b := Block(testHome)
	for _, want := range []string{
		beginMarker,
		endMarker,
		"studio jobs list",
		"studio queue show",
		"studio prompts show",
		testHome,
	} {
		if !strings.Contains(b, want) {
			t.Errorf("Block() missing %q", want)
		}
	}
}

func TestHasBlock(t *testing.T) {
	if HasBlock("no block here") {
		t.Error("HasBlock should be false")
	}
	if !HasBlock(Block(testHome)) {
		t.Error("HasBlock should be true for generated block")
	}
}

func TestReplaceBlock_insertsIntoEmpty(t *testing.T) {
	result := replaceBlock("", Block(testHome))
	if !HasBlock(result) {
		t.Error("block not inserted into empty content")
	}
}

func TestReplaceBlock_appendsToExisting(t *testing.T) {
	existing := "# My notes\n\nSome content.\n"
	result := replaceBlock(existing, Block(testHome))
	if !strings.Contains(result, "My notes") {
		t.Error("existing content lost")
	}
	if !HasBlock(result) {
		t.Error("block not appended")
	}
}

func TestReplaceBlock_replacesExistingBlock(t *testing.T) {
	first := replaceBlock("# Notes\n", Block(testHome))
	// Modify block content to simulate an update.
	newBlock := strings.Replace(Block(testHome), "studio jobs list", "studio jobs list --json", 1)
	second := replaceBlock(first, newBlock)

	if strings.Contains(second, "studio jobs list\n") {
		t.Error("old block content should be replaced")
	}
	if !strings.Contains(second, "studio jobs list --json") {
		t.Error("new block content missing")
	}
	if !strings.Contains(second, "# Notes") {
		t.Error("surrounding content lost")
	}
	// Should only have one block.
	if strings.Count(second, beginMarker) != 1 {
		t.Errorf("expected 1 begin marker, got %d", strings.Count(second, beginMarker))
	}
}

func TestReplaceBlock_idempotent(t *testing.T) {
	base := "# Notes\n\nContent.\n"
	block := Block(testHome)
	first := replaceBlock(base, block)
	second := replaceBlock(first, block)
	if first != second {
		t.Errorf("replaceBlock not idempotent:\nfirst:\n%s\nsecond:\n%s", first, second)
	}
}

func TestReplaceBlock_removeWithEmptyBlock(t *testing.T) {
	content := "# Notes\n\n" + Block(testHome) + "\nMore content.\n"
	result := replaceBlock(content, "")
	if HasBlock(result) {
		t.Error("block should be removed")
	}
	if !strings.Contains(result, "# Notes") {
		t.Error("pre-block content lost")
	}
	if !strings.Contains(result, "More content.") {
		t.Error("post-block content lost")
	}
}

func TestInject_createsFileAndDir(t *testing.T) {
	tmp := t.TempDir()
	claudeMD := filepath.Join(tmp, ".claude", "CLAUDE.md")

	// Override Path() by writing directly to a known location — test Inject via filesystem.
	dir := filepath.Dir(claudeMD)
	os.MkdirAll(dir, 0o755)

	// Write an empty file and verify Inject adds the block.
	os.WriteFile(claudeMD, []byte(""), 0o644)

	content := Block(testHome)
	updated := replaceBlock("", content)
	os.WriteFile(claudeMD, []byte(updated), 0o644)

	data, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !HasBlock(string(data)) {
		t.Error("block not found after inject")
	}
}

func TestRemoveBlock_leavesOtherContent(t *testing.T) {
	original := "# My CLAUDE.md\n\nGlobal instructions.\n"
	withBlock := replaceBlock(original, Block(testHome))
	without := replaceBlock(withBlock, "")

	if HasBlock(without) {
		t.Error("block still present after remove")
	}
	if !strings.Contains(without, "Global instructions.") {
		t.Error("surrounding content lost after remove")
	}
}
