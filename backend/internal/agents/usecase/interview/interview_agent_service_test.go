package interview

import "testing"

func TestThinkContentFilter_RemovesThinkTagsInSingleChunk(t *testing.T) {
	output := runFilter([]string{"Hello<think>hidden reasoning</think>World"})
	if output != "HelloWorld" {
		t.Fatalf("expected %q, got %q", "HelloWorld", output)
	}
}

func TestThinkContentFilter_RemovesThinkTagsAcrossChunks(t *testing.T) {
	chunks := []string{"Hello<thi", "nk>hidden", " text</thinking>", "World"}
	output := runFilter(chunks)
	if output != "HelloWorld" {
		t.Fatalf("expected %q, got %q", "HelloWorld", output)
	}
}

func TestThinkContentFilter_DropsUnclosedThinkBlock(t *testing.T) {
	output := runFilter([]string{"Hello<think>hidden forever"})
	if output != "Hello" {
		t.Fatalf("expected %q, got %q", "Hello", output)
	}
}

func TestThinkContentFilter_KeepNormalContent(t *testing.T) {
	output := runFilter([]string{"Question: ", "Please explain GC in Go."})
	if output != "Question: Please explain GC in Go." {
		t.Fatalf("unexpected output: %q", output)
	}
}

func runFilter(chunks []string) string {
	f := newThinkContentFilter()
	out := ""
	for _, c := range chunks {
		out += f.Push(c)
	}
	out += f.Flush()
	return out
}
