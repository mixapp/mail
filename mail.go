package mail

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/gomail.v2"
)

type SmtpClient struct {
	Host     string
	Port     string
	User     string
	Password string
	From     string
	TLS      bool
}

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
	BodyContentType string
	Attachments     map[string]*Attachment
}

func NewMessage(smtpClient *SmtpClient, to string, subject string, body string) *Message {

	return &Message{
		smtpClient:      *smtpClient,
		Subject:         subject,
		To:              to,
		Body:            body,
		BodyContentType: "text/html",
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

	header := Header{}
	header.Set("MIME-Version", "1.0")
	header.Set("Subject", m.Subject)
	header.SetDate("Date", time.Now())
	header.SetAddress("From", m.smtpClient.From)
	header.SetAddress("To", m.To)

	if len(m.Cc) > 0 {
		header.SetAddress("Cc", m.Cc...)
	}

	if len(m.Bcc) > 0 {
		header.SetAddress("Bcc", m.Bcc...)
	}

	if len(m.ReplyTo) > 0 {
		header.SetAddress("Reply-To", m.ReplyTo)
	}

	data := bytes.NewBuffer(nil)
	body := []byte(m.Body)

	switch len(m.Attachments) {
	case 0:
		header.Set("Content-Type", getContentType(body))
		data.Write(body)

	default:

		multipartWriter := multipart.NewWriter(data)
		header.SetValue("Content-Type", "multipart/mixed", HeaderParams{"boundary": multipartWriter.Boundary()})

		if err := attachData(multipartWriter, body, true, ""); err != nil {
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

	msg := bytes.NewBuffer(header.Bytes())
	msg.WriteString("\r\n")
	msg.Write(data.Bytes())

	servername := fmt.Sprintf("%s:%s", m.smtpClient.Host, m.smtpClient.Port)
	host, _, _ := net.SplitHostPort(servername)
	auth := smtp.PlainAuth("", m.smtpClient.User, m.smtpClient.Password, host)

	// Connect to the server, authenticate, set the sender and recipient,
	// and send the email all in one step.
	err := smtp.SendMail(servername, auth, m.smtpClient.From, []string{m.To}, msg.Bytes())
	if err != nil {
		return err
	}

	//TODO проверить

	if !m.smtpClient.TLS {

		mail := gomail.NewMessage()
		mail.SetHeader("From", m.smtpClient.From)
		mail.SetHeader("To", m.To)
		mail.SetHeader("Subject", m.Subject)
		mail.SetBody(m.BodyContentType, m.Body)

		port, _ := strconv.Atoi(m.smtpClient.Port)
		d := gomail.NewDialer(m.smtpClient.Host, port, m.smtpClient.User, m.smtpClient.Password)
		d.TLSConfig = &tls.Config{InsecureSkipVerify: true}

		err := d.DialAndSend(mail)
		return err
	}

	/* Актуально для TLS */

	tlsconfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         host,
	}

	dl := new(net.Dialer)
	dl.Timeout = 10 * time.Second
	conn, err := tls.DialWithDialer(dl, "tcp", servername, tlsconfig)

	//conn, err := tls.Dial("tcp", servername, tlsconfig)
	if err != nil {
		return err
	}

	c, err := smtp.NewClient(conn, host)
	defer func() {
		c.Quit()
	}()
	if err != nil {
		return err
	}

	// Auth
	if err = c.Auth(auth); err != nil {
		return err
	}

	// To && From
	emailFrom, err := mail.ParseAddress(m.smtpClient.From)
	if err != nil {
		return err
	}

	if err = c.Mail(emailFrom.Address); err != nil {
		return err
	}

	if err = c.Rcpt(m.To); err != nil {
		return err
	}

	// Data
	w, err := c.Data()
	if err != nil {
		return err
	}

	// _, err = w.Write(buf.Bytes())
	// if err != nil {
	// 	return err
	// }

	err = w.Close()
	if err != nil {
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
	header.Set("Content-Transfer-Encoding", "base64")
	header.SetValue("Content-Type", getContentType(src), contentTypeParams)
	header.SetValue("Content-Disposition", dispositionType, dispositionParams)

	if len(contentDescription) > 0 {
		header.Set("Content-Description", contentDescription)
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
