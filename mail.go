package mail

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"mime"
	"net"
	"net/smtp"
	"path/filepath"
	"strings"
	"time"
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

	buf := bytes.NewBuffer(nil)
	buf.WriteString("From: " + m.smtpClient.From + "\r\n")
	buf.WriteString("To: " + m.To + "\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Date: " + time.Now().Format(time.RFC822) + "\r\n")

	if len(m.Cc) > 0 {
		buf.WriteString("Cc: " + strings.Join(m.Cc, ",") + "\r\n")
	}

	buf.WriteString("Subject: " + m.Subject + "\r\n")

	if len(m.ReplyTo) > 0 {
		buf.WriteString("Reply-To: " + m.ReplyTo + "\r\n")
	}

	/* Генерация тела сообщения */
	boundary := "f46d043c813270fc6b04c2d223da"

	if len(m.Attachments) > 0 {
		buf.WriteString("Content-Type: multipart/mixed; boundary=" + boundary + "\r\n\r\n")
		buf.WriteString("--" + boundary + "\r\n")
		buf.WriteString("Content-Transfer-Encoding: 8bit\r\n")
		buf.WriteString("Content-Type: " + m.BodyContentType + "; charset=utf-8\r\n\r\n")
		buf.WriteString(m.Body)
		buf.WriteString("\r\n")
	} else {
		buf.WriteString("Content-Type: " + m.BodyContentType + "; charset=utf-8\r\n\r\n")
		buf.WriteString(m.Body)
		buf.WriteString("\r\n")
	}

	if len(m.Attachments) > 0 {
		for _, attachment := range m.Attachments {
			buf.WriteString("\r\n\r\n--" + boundary + "\r\n")

			if attachment.Inline {
				buf.WriteString("Content-Type: message/rfc822\r\n")
				buf.WriteString("Content-Disposition: inline; filename=\"" + mime.QEncoding.Encode("utf-8", attachment.Filename) + "\"\r\n\r\n")

				buf.Write(attachment.Data)
			} else {
				buf.WriteString("Content-Disposition: attachment; filename=\"" + mime.QEncoding.Encode("utf-8", attachment.Filename) + "\"\r\n")
				buf.WriteString("Content-Transfer-Encoding: base64\r\n")
				buf.WriteString("Content-Type: application/octet-stream; name=\"" + mime.QEncoding.Encode("utf-8", attachment.Filename) + "\"\r\n\r\n")

				b := make([]byte, base64.StdEncoding.EncodedLen(len(attachment.Data)))
				base64.StdEncoding.Encode(b, attachment.Data)

				// write base64 content in lines of up to 76 chars
				for i, l := 0, len(b); i < l; i++ {
					buf.WriteByte(b[i])
					if (i+1)%76 == 0 {
						buf.WriteString("\r\n")
					}
				}
			}

			buf.WriteString("\r\n--" + boundary)
		}

		buf.WriteString("--")
	}

	/*=======================================*/
	servername := fmt.Sprintf("%s:%s", m.smtpClient.Host, m.smtpClient.Port)
	host, _, _ := net.SplitHostPort(servername)
	auth := smtp.PlainAuth("", m.smtpClient.User, m.smtpClient.Password, host)

	if !m.smtpClient.TLS {
		err := smtp.SendMail(servername, auth, m.smtpClient.From, []string{m.To}, buf.Bytes())
		if err != nil {
			return err
		}

		return nil
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
	if err = c.Mail(m.smtpClient.From); err != nil {
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

	_, err = w.Write(buf.Bytes())
	if err != nil {
		return err
	}

	err = w.Close()
	if err != nil {
		return err
	}

	return nil
}
