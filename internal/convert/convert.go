package convert

import (
	"fmt"
	"strings"

	"github.com/jomei/notionapi"
)

// Blocks converts a slice of Notion blocks to clean markdown.
func Blocks(blocks []notionapi.Block) string {
	var sb strings.Builder
	numCounter := 0

	for _, block := range blocks {
		if block.GetType() != notionapi.BlockTypeNumberedListItem {
			numCounter = 0
		}
		switch b := block.(type) {
		case *notionapi.ParagraphBlock:
			if text := richText(b.Paragraph.RichText); text != "" {
				fmt.Fprintf(&sb, "%s\n\n", text)
			}
		case *notionapi.Heading1Block:
			fmt.Fprintf(&sb, "# %s\n\n", richText(b.Heading1.RichText))
		case *notionapi.Heading2Block:
			fmt.Fprintf(&sb, "## %s\n\n", richText(b.Heading2.RichText))
		case *notionapi.Heading3Block:
			fmt.Fprintf(&sb, "### %s\n\n", richText(b.Heading3.RichText))
		case *notionapi.BulletedListItemBlock:
			fmt.Fprintf(&sb, "- %s\n", richText(b.BulletedListItem.RichText))
		case *notionapi.NumberedListItemBlock:
			numCounter++
			fmt.Fprintf(&sb, "%d. %s\n", numCounter, richText(b.NumberedListItem.RichText))
		case *notionapi.ToDoBlock:
			check := "[ ]"
			if b.ToDo.Checked {
				check = "[x]"
			}
			fmt.Fprintf(&sb, "- %s %s\n", check, richText(b.ToDo.RichText))
		case *notionapi.ToggleBlock:
			fmt.Fprintf(&sb, "- %s\n", richText(b.Toggle.RichText))
		case *notionapi.CodeBlock:
			lang := string(b.Code.Language)
			if lang == "plain text" {
				lang = ""
			}
			fmt.Fprintf(&sb, "```%s\n%s\n```\n\n", lang, richText(b.Code.RichText))
		case *notionapi.QuoteBlock:
			fmt.Fprintf(&sb, "> %s\n\n", richText(b.Quote.RichText))
		case *notionapi.CalloutBlock:
			icon := calloutIcon(b.Callout.Icon)
			fmt.Fprintf(&sb, "> %s%s\n\n", icon, richText(b.Callout.RichText))
		case *notionapi.DividerBlock:
			sb.WriteString("---\n\n")
		case *notionapi.ImageBlock:
			url := fileObjectURL(b.Image.File, b.Image.External)
			caption := richText(b.Image.Caption)
			if caption == "" {
				caption = "image"
			}
			fmt.Fprintf(&sb, "![%s](%s)\n\n", caption, url)
		case *notionapi.BookmarkBlock:
			caption := richText(b.Bookmark.Caption)
			if caption == "" {
				caption = b.Bookmark.URL
			}
			fmt.Fprintf(&sb, "[%s](%s)\n\n", caption, b.Bookmark.URL)
		case *notionapi.ChildPageBlock:
			fmt.Fprintf(&sb, "- [[%s]]\n", b.ChildPage.Title)
		}
	}

	return strings.TrimSpace(sb.String())
}

// Title extracts the plain-text title from a page's properties.
func Title(props notionapi.Properties) string {
	for _, prop := range props {
		if tp, ok := prop.(*notionapi.TitleProperty); ok && len(tp.Title) > 0 {
			return richText(tp.Title)
		}
	}
	return "Untitled"
}

func richText(rts []notionapi.RichText) string {
	var sb strings.Builder
	for _, rt := range rts {
		text := rt.PlainText
		if rt.Annotations != nil {
			switch {
			case rt.Annotations.Code:
				text = "`" + text + "`"
			default:
				if rt.Annotations.Bold {
					text = "**" + text + "**"
				}
				if rt.Annotations.Italic {
					text = "_" + text + "_"
				}
				if rt.Annotations.Strikethrough {
					text = "~~" + text + "~~"
				}
			}
		}
		if rt.Href != "" {
			text = fmt.Sprintf("[%s](%s)", text, rt.Href)
		}
		sb.WriteString(text)
	}
	return sb.String()
}

func calloutIcon(icon *notionapi.Icon) string {
	if icon == nil || icon.Emoji == nil {
		return ""
	}
	return string(*icon.Emoji) + " "
}

func fileObjectURL(file, external *notionapi.FileObject) string {
	if external != nil && external.URL != "" {
		return external.URL
	}
	if file != nil && file.URL != "" {
		return file.URL
	}
	return ""
}
