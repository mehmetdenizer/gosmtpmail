package gosmtpmail

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/mehmetdenizer/gohelpers"
	"mime"
	"mime/multipart"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
)

type EmailConfig struct {
	EmailAddress         string
	Password             string
	Host                 string
	Port                 string
	SenderName           string
	ReplyTo              string
	AttachmentPathPrefix string
	BccAddressToSendCopy string
}

var emailConfig EmailConfig

func SetConfig(config EmailConfig) {
	emailConfig = config
}

// EmailSender sends an email
func EmailSender(subject, body, htmlBody, attachmentPath string, to []string) bool {
	// Define Auth
	auth := emailAuth()

	// Append BCC address if it's not empty
	recipients := to
	if emailConfig.BccAddressToSendCopy != "" {
		recipients = append(recipients, emailConfig.BccAddressToSendCopy)
	}

	// Create message
	message, e := createEmailMessage(subject, body, htmlBody, attachmentPath, to)
	if e != nil {
		gohelpers.LogError("Error creating message:", e)
		return false
	}

	// Send mail
	err := smtp.SendMail(
		emailConfig.Host+":"+emailConfig.Port,
		auth,
		emailConfig.EmailAddress,
		recipients,
		message)
	if err != nil {
		gohelpers.LogError("Error sending email:", err)
		return false
	}
	return true
}

// emailAuth returns smtp.Auth type
func emailAuth() smtp.Auth {
	return smtp.PlainAuth("", emailConfig.EmailAddress, emailConfig.Password, emailConfig.Host)
}

// encodeHeader encodes header in base64
func encodeHeader(header string) string {
	return fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(header)))
}

// createEmailMessage creates an email message with an attachment
func createEmailMessage(subject, body, htmlBody, attachmentPath string, to []string) ([]byte, error) {
	// Check if attachment path starts with "storage/" ("storage/" is an example)
	prefix := emailConfig.AttachmentPathPrefix + "/"
	if attachmentPath != "" && !strings.HasPrefix(attachmentPath, prefix) {
		return nil, errors.New("attachment path must start with: " + prefix)
	}

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Headers
	boundary := writer.Boundary()
	headers := fmt.Sprintf("MIME-Version: 1.0\r\nFrom: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nReply-To: %s\r\nContent-Type: multipart/mixed; boundary=%s\r\n\r\n",
		encodeHeader(emailConfig.SenderName),
		emailConfig.EmailAddress,
		strings.Join(to, ", "),
		encodeHeader(subject),
		emailConfig.ReplyTo,
		boundary)
	buf.Write([]byte(headers))

	// Body part
	if body != "" && htmlBody != "" {
		// If both text and HTML are provided
		altWriter := multipart.NewWriter(&buf)
		altBoundary := altWriter.Boundary()
		buf.Write([]byte(fmt.Sprintf("--%s\r\nContent-Type: multipart/alternative; boundary=%s\r\n\r\n", boundary, altBoundary)))

		// Plain text part
		textHeader := textproto.MIMEHeader{}
		textHeader.Set("Content-Type", "text/plain; charset=UTF-8")
		textPart, err := altWriter.CreatePart(textHeader)
		if err != nil {
			return nil, err
		}
		_, err = textPart.Write([]byte(body))
		if err != nil {
			return nil, err
		}

		// HTML part
		htmlHeader := textproto.MIMEHeader{}
		htmlHeader.Set("Content-Type", "text/html; charset=UTF-8")
		htmlPart, err := altWriter.CreatePart(htmlHeader)
		if err != nil {
			return nil, err
		}
		_, err = htmlPart.Write([]byte(htmlBody))
		if err != nil {
			return nil, err
		}

		err = altWriter.Close()
		if err != nil {
			return nil, err
		}
	} else if body != "" {
		// If only text is provided
		textHeader := textproto.MIMEHeader{}
		textHeader.Set("Content-Type", "text/plain; charset=UTF-8")
		textPart, err := writer.CreatePart(textHeader)
		if err != nil {
			return nil, err
		}
		_, err = textPart.Write([]byte(body))
		if err != nil {
			return nil, err
		}
	} else if htmlBody != "" {
		// If only HTML is provided
		htmlHeader := textproto.MIMEHeader{}
		htmlHeader.Set("Content-Type", "text/html; charset=UTF-8")
		htmlPart, err := writer.CreatePart(htmlHeader)
		if err != nil {
			return nil, err
		}
		_, err = htmlPart.Write([]byte(htmlBody))
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("neither body nor htmlBody provided")
	}

	// Attachment part
	if attachmentPath != "" {
		attachment, err := os.ReadFile(attachmentPath)
		if err != nil {
			return nil, err
		}
		attachmentHeader := textproto.MIMEHeader{}
		attachmentHeader.Set("Content-Type", mime.TypeByExtension(filepath.Ext(attachmentPath)))
		attachmentHeader.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(attachmentPath)))
		attachmentHeader.Set("Content-Transfer-Encoding", "base64")
		attachmentPart, err := writer.CreatePart(attachmentHeader)
		if err != nil {
			return nil, err
		}
		encoded := base64.StdEncoding.EncodeToString(attachment)
		_, err = attachmentPart.Write([]byte(encoded))
		if err != nil {
			return nil, err
		}
	}

	err := writer.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
