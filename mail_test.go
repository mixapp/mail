package mail

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"strings"
	"testing"
	"time"
)

func TestSend(t *testing.T) {

	SMTP := getSmtpClient()
	to := "postmaster@example.org"

	attachment1 := Attachment{
		Filename: "Текст",
		Data:     []byte("Тестовое сообщение"),
		Inline:   true,
	}

	attachment2 := Attachment{
		Filename: "Image",
		Data:     createImage(),
		Inline:   true,
	}

	subjectDesc := time.Now().Format(time.RFC3339)

	for testIndex := 0; testIndex < 6; testIndex++ {
		var body string
		var attachments map[string]*Attachment

		switch testIndex {
		case 0:
			body = fmt.Sprintf("Тестовое сообщение (text/plain): \n%s", SMTP.User)
		case 1:
			body = fmt.Sprintf("<html><body><b>Тестовое сообщение (text/html): <br/>%s </b></body></html>", SMTP.User)
		case 2:
			body = "Inline attachment (format: text)"
			attachments = map[string]*Attachment{
				attachment1.Filename: &attachment1,
			}
		case 3:
			body = "Inline attachment (format: image/jpeg)"
			attachments = map[string]*Attachment{
				attachment2.Filename: &attachment2,
			}
		case 4:
			body = "Inline attachments (format: text & image/jpeg)"
			attachments = map[string]*Attachment{
				attachment1.Filename: &attachment1,
				attachment2.Filename: &attachment2,
			}
		case 5:
			body = "Attachments (format: text & image/jpeg)"
			a1 := attachment1
			a1.Inline = false

			a2 := attachment2
			a2.Inline = false

			attachments = map[string]*Attachment{
				a1.Filename: &a1,
				a2.Filename: &a2,
			}
		default:
			t.Fatal("Fail test.")
		}

		delimeter := ","
		if testIndex%2 == 1 {
			delimeter = ";"
		}

		msg := Message{
			smtpClient:  *SMTP,
			To:          strings.Join([]string{to, to, to}, delimeter),
			Cc:          []string{to, to},
			Bcc:         []string{to, to},
			ReplyTo:     to,
			Subject:     fmt.Sprintf("Test №%d: %s", testIndex, subjectDesc),
			Body:        body,
			Attachments: attachments,
		}

		if err := msg.SendMail(); err != nil {
			t.Errorf("Test №%d: %s", testIndex, err.Error())
		} else {
			fmt.Println(fmt.Sprintf("Test №%d: ok", testIndex))
		}
	}

}

func createImage() []byte {

	tmpImage := image.NewRGBA(image.Rect(0, 0, 32, 32))
	blue := color.RGBA{0, 0, 255, 255}
	draw.Draw(tmpImage, tmpImage.Bounds(), &image.Uniform{blue}, image.ZP, draw.Src)

	// Write white line
	for i := tmpImage.Bounds().Min.X; i < tmpImage.Bounds().Max.X; i++ {
		tmpImage.Set(i, tmpImage.Bounds().Max.Y/2, color.RGBA{255, 255, 255, 255})
	}

	out := bytes.NewBuffer(nil)

	opt := jpeg.Options{Quality: 100}
	if err := jpeg.Encode(out, tmpImage, &opt); err != nil {
		panic(err)
	}

	return out.Bytes()
}
