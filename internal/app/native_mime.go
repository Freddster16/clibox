package app

import (
	"bytes"
	"errors"
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
				images = appendMessageImage(images, messageImage{
					Name:        firstNonEmpty(name, "inline image"),
					ContentType: mediaType,
					Data:        body,
				})
			}
		}
	}
	if text := strings.TrimSpace(strings.Join(nonEmpty(plainParts...), "\n\n")); text != "" {
		return messageContent{Body: text, Images: images}
	}
	if text := strings.TrimSpace(strings.Join(nonEmpty(htmlParts...), "\n\n")); text != "" {
		return messageContent{Body: text, Images: images}
	}
	return messageContent{Body: normalizeMessageBody(raw), Images: images}
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

func htmlToText(input string) string {
	input = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`).ReplaceAllString(input, "")
	input = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`).ReplaceAllString(input, "")
	input = regexp.MustCompile(`(?i)<br\s*/?>`).ReplaceAllString(input, "\n")
	input = regexp.MustCompile(`(?i)</p\s*>`).ReplaceAllString(input, "\n\n")
	input = regexp.MustCompile(`(?s)<[^>]+>`).ReplaceAllString(input, " ")
	input = html.UnescapeString(input)
	return strings.Join(strings.Fields(input), " ")
}
