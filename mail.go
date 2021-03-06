package mail

import (
	"bytes"
	"encoding/base64"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	netmail "net/mail"
	"path/filepath"
	"strings"
	"time"
)

type Attachment struct {
	Filename string
	Data     []byte
	Inline   bool
}

type Message struct {
	smtpClient      SmtpClient
	To              string
	Cc              []string
	Bcc             []string
	ReplyTo         string
	Subject         string
	Body            string
	BodyContentType string // TODO remove, because don't use in the library
	Attachments     map[string]*Attachment
}

func NewMessage(smtpClient *SmtpClient, to string, subject string, body string) *Message {

	return &Message{
		smtpClient:      *smtpClient,
		Subject:         subject,
		To:              to,
		Body:            body,
		BodyContentType: getContentType([]byte(body)), // TODO remove, because don't use in the library
		Attachments:     make(map[string]*Attachment),
	}
}

func (m *Message) Attach(file string, inline bool) error {

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	_, filename := filepath.Split(file)

	m.Attachments[filename] = &Attachment{
		Filename: filename,
		Data:     data,
		Inline:   inline,
	}

	return nil
}

func (m *Message) SendMail() error {

	// MESSAGE HEADER

	header := Header{}
	header.SetString("MIME-Version", "1.0")
	header.SetString("Subject", m.Subject)
	header.SetDate("Date", time.Now())

	if err := header.SetAddress("From", m.smtpClient.From); err != nil {
		return err
	}

	if err := header.SetAddress("To", m.To); err != nil {
		return err
	}

	if len(m.Cc) > 0 {
		if err := header.SetAddress("Cc", m.Cc...); err != nil {
			return err
		}
	}

	if len(m.Bcc) > 0 {
		if err := header.SetAddress("Bcc", m.Bcc...); err != nil {
			return err
		}
	}

	if len(m.ReplyTo) > 0 {
		if err := header.SetAddress("Reply-To", m.ReplyTo); err != nil {
			return err
		}
	}

	// MESSAGE BODY

	body := bytes.NewBuffer(nil)
	bodySrc := []byte(m.Body)

	switch len(m.Attachments) {
	case 0:
		header.SetString("Content-Type", getContentType(bodySrc))
		body.Write(bodySrc)

	default:

		multipartWriter := multipart.NewWriter(body)
		header.SetValue("Content-Type", "multipart/mixed", HeaderParams{"boundary": multipartWriter.Boundary()})

		if err := attachData(multipartWriter, bodySrc, true, ""); err != nil {
			return err
		}

		for _, attachment := range m.Attachments {
			if err := attachData(multipartWriter, attachment.Data, attachment.Inline, attachment.Filename); err != nil {
				return err
			}
		}

		if err := multipartWriter.Close(); err != nil {
			return err
		}
	}

	// SEND MESSAGE
	c, err := m.smtpClient.Connection()
	if err != nil {
		return err
	}

	if e, err := netmail.ParseAddress(m.smtpClient.From); err != nil {
		return err
	} else if err = c.Mail(e.Address); err != nil {
		return err
	}

	recipientsList := []string{m.To}
	if m.Cc != nil {
		recipientsList = append(recipientsList, strings.Join(m.Cc, ","))
	}
	if m.Bcc != nil {
		recipientsList = append(recipientsList, strings.Join(m.Bcc, ","))
	}

	for _, recipients := range recipientsList {

		if emails, err := parseAdresses(recipients); err != nil {
			return err
		} else {
			for _, e := range emails {
				if err = c.Rcpt(e.Address); err != nil {
					return err
				}
			}
		}
	}

	msg := bytes.NewBuffer(header.Bytes())
	msg.WriteString("\r\n")
	msg.Write(body.Bytes())

	if w, err := c.Data(); err != nil {
		return err
	} else if _, err = w.Write(msg.Bytes()); err != nil {
		return err
	} else if err = w.Close(); err != nil {
		return err
	}

	return nil
}

func attachData(multipartWriter *multipart.Writer, src []byte, inline bool, filename string) error {

	// Example:
	// 	--3379bd9a9b4ba7731a26ac044694f6f30cd02b0f9fd0c9a123531d163625
	// Content-Description: .gitignore
	// Content-Disposition: inline; filename=".gitignore"
	// Content-Transfer-Encoding: base64
	// Content-Type: text/html; charset=utf-8; name=".gitignore"

	// PGh0bWw+PC9odG1sPg==
	// --3379bd9a9b4ba7731a26ac044694f6f30cd02b0f9fd0c9a123531d163625--

	var (
		contentDescription                   string
		contentTypeParams, dispositionParams = HeaderParams{}, HeaderParams{}
	)

	data := []byte(base64.StdEncoding.EncodeToString(src))

	dispositionType := "attachment"
	if inline {
		dispositionType = "inline"
	}

	if len(filename) > 0 {
		dispositionParams["filename"] = filename
		contentTypeParams["name"] = filename
		contentDescription = filename
	}

	header := Header{}
	header.SetString("Content-Transfer-Encoding", "base64")
	header.SetValue("Content-Type", getContentType(src), contentTypeParams)
	header.SetValue("Content-Disposition", dispositionType, dispositionParams)

	if len(contentDescription) > 0 {
		header.SetString("Content-Description", contentDescription)
	}

	if part, err := multipartWriter.CreatePart(header.MIMEHeader()); err != nil {
		return err
	} else {
		part.Write(data)
	}

	return nil
}

func getQEncodeString(src string) string {
	return mime.QEncoding.Encode("utf-8", src)
}

func getContentType(src []byte) string {
	return http.DetectContentType(src)
}

var delimeterReplacer = strings.NewReplacer(";", ",")

func parseAdresses(src string) ([]*netmail.Address, error) {
	src = delimeterReplacer.Replace(src)
	return netmail.ParseAddressList(src)
}
