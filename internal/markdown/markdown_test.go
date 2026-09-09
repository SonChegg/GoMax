package markdown

import "testing"

func TestFormatBoldAndLink(t *testing.T) {
	clean, elements := Format("**hi** [click](https://example.com)")

	if clean != "hi click" {
		t.Fatalf("unexpected clean text: %q", clean)
	}

	if len(elements) != 2 {
		t.Fatalf("expected 2 elements, got %d: %+v", len(elements), elements)
	}

	strong := elements[0]
	if strong.Type != "STRONG" || strong.From == nil || *strong.From != 0 || strong.Length == nil || *strong.Length != 2 {
		t.Fatalf("unexpected STRONG element: %+v", strong)
	}

	link := elements[1]
	if link.Type != "LINK" || link.Attributes == nil || link.Attributes.URL == nil || *link.Attributes.URL != "https://example.com" {
		t.Fatalf("unexpected LINK element: %+v", link)
	}
}

func TestFormatPlainTextHasNoElements(t *testing.T) {
	clean, elements := Format("just plain text")
	if clean != "just plain text" {
		t.Fatalf("unexpected clean text: %q", clean)
	}
	if len(elements) != 0 {
		t.Fatalf("expected no elements, got %+v", elements)
	}
}

func TestFormatUnmatchedMarkerIsLiteral(t *testing.T) {
	clean, elements := Format("cost is $5 * 3 apples")
	if clean != "cost is $5 * 3 apples" {
		t.Fatalf("unexpected clean text: %q", clean)
	}
	if len(elements) != 0 {
		t.Fatalf("expected no elements for unmatched marker, got %+v", elements)
	}
}
