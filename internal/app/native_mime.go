package app

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"io"
	"regexp"
	"strings"

	gomessage "github.com/emersion/go-message"
	mailmessage "github.com/emersion/go-message/mail"
)

const (
	maxMessageImages    = 1
	maxInlineImageBytes = 2 << 20
	maxMessageBodyBytes = 20 << 20
)

func extractReadableMessageBody(raw []byte) string {
	return extractReadableMessageContent(raw).Body
}

func extractReadableMessageContent(raw []byte) messageContent {
	if len(bytes.TrimSpace(raw)) == 0 {
		return messageContent{}
	}
	reader, err := mailmessage.CreateReader(bytes.NewReader(raw))
	if err != nil {
		return messageContent{Body: normalizeMessageBody(raw)}
	}
	var plainParts []string
	var htmlParts []string
	var images []messageImage
	droppedImages := 0
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil && !gomessage.IsUnknownCharset(err) {
			break
		}
		if part == nil || part.Body == nil {
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(part.Body, 4<<20))
		if readErr != nil {
			continue
		}
		mediaType, name := partContentInfo(part.Header)
		switch strings.ToLower(mediaType) {
		case "text/plain", "":
			plainParts = append(plainParts, normalizeMessageBody(body))
		case "text/html":
			htmlParts = append(htmlParts, htmlToText(string(body)))
		default:
			if strings.HasPrefix(strings.ToLower(mediaType), "image/") {
				if len(images) >= maxMessageImages || len(body) > maxInlineImageBytes {
					droppedImages++
				} else {
					images = appendMessageImage(images, messageImage{
						Name:        firstNonEmpty(name, "inline image"),
						ContentType: mediaType,
						Data:        body,
					})
				}
			}
		}
	}
	notice := ""
	if droppedImages > 0 {
		notice = fmt.Sprintf("%d inline image(s) were not shown (size or count limit reached).", droppedImages)
	}
	if text := strings.TrimSpace(strings.Join(nonEmpty(plainParts...), "\n\n")); text != "" {
		return messageContent{Body: text, Images: images, Notice: notice}
	}
	if text := strings.TrimSpace(strings.Join(nonEmpty(htmlParts...), "\n\n")); text != "" {
		return messageContent{Body: text, Images: images, Notice: notice}
	}
	return messageContent{Body: normalizeMessageBody(raw), Images: images, Notice: notice}
}

func partContentInfo(header any) (string, string) {
	switch header := header.(type) {
	case *mailmessage.InlineHeader:
		mediaType, params, _ := header.ContentType()
		_, dispositionParams, _ := header.ContentDisposition()
		return strings.ToLower(mediaType), firstNonEmpty(dispositionParams["filename"], params["name"], headerText(header.Header, "Content-Description"))
	case *mailmessage.AttachmentHeader:
		mediaType, params, _ := header.ContentType()
		filename, _ := header.Filename()
		return strings.ToLower(mediaType), firstNonEmpty(filename, params["name"], headerText(header.Header, "Content-Description"))
	}
	return "", ""
}

func headerText(header gomessage.Header, key string) string {
	value, err := header.Text(key)
	if err != nil {
		return ""
	}
	return value
}

func appendMessageImage(images []messageImage, image messageImage) []messageImage {
	if len(images) >= maxMessageImages || len(image.Data) == 0 || len(image.Data) > maxInlineImageBytes {
		return images
	}
	image.Name = terminalSafeLine(firstNonEmpty(image.Name, "image"))
	image.ContentType = strings.ToLower(firstNonEmpty(image.ContentType, "image"))
	return append(images, image)
}

var (
	htmlScriptRE    = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	htmlStyleRE     = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	htmlParaCloseRE = regexp.MustCompile(`(?i)</(p|h[1-6]|blockquote|div|section|article|header|footer|pre)>`)
	htmlLineCloseRE = regexp.MustCompile(`(?i)</(li|tr|ul|ol|table)>`)
	htmlBrRE        = regexp.MustCompile(`(?i)<br\s*/?>`)
	htmlTagRE       = regexp.MustCompile(`(?s)<[^>]+>`)
	htmlBlankLinRE  = regexp.MustCompile(`\n{3,}`)
)

func htmlToText(input string) string {
	input = htmlScriptRE.ReplaceAllString(input, "")
	input = htmlStyleRE.ReplaceAllString(input, "")
	input = htmlParaCloseRE.ReplaceAllString(input, "\n\n")
	input = htmlLineCloseRE.ReplaceAllString(input, "\n")
	input = htmlBrRE.ReplaceAllString(input, "\n")
	input = htmlTagRE.ReplaceAllString(input, " ")
	input = html.UnescapeString(input)
	return normalizeHTMLText(input)
}

func normalizeHTMLText(input string) string {
	lines := strings.Split(input, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		out = append(out, line)
	}
	normalized := htmlBlankLinRE.ReplaceAllString(strings.Join(out, "\n"), "\n\n")
	return strings.TrimSpace(normalized)
}
