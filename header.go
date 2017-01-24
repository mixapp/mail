package mail

import (
	"bytes"
	"fmt"
	"net/mail"
	"net/textproto"
	"strings"
	"time"
)

const TIME_FORMAT = time.RFC1123Z

type Header textproto.MIMEHeader
type HeaderParams map[string]interface{}

func (h Header) MIMEHeader() textproto.MIMEHeader {
	return textproto.MIMEHeader(h)
}

func (h Header) Set(key string, value string) {
	h.set(key, getQEncodeString(value))
}

func (h Header) SetDate(key string, value time.Time) {
	h.set(key, value.Format(TIME_FORMAT))
}

func (h Header) SetAddress(key string, value ...string) error {

	buf := bytes.NewBuffer(nil)
	for _, a := range value {
		if e, err := mail.ParseAddress(a); err != nil {
			return err
		} else {
			if buf.Len() > 0 {
				buf.WriteString(",")
			}
			buf.WriteString(e.String())
		}
	}

	// result examples:
	// - box@gmail.com
	// - Alias <box@gmail.com>
	// - box1@gmail.com; box2@gmail.com
	// - Alias <box1@gmail.com>; box2@gmail.com

	h.set(key, buf.String())
	return nil
}

func (h Header) SetValue(key, value string, params HeaderParams) {

	buf := bytes.NewBufferString(getQEncodeString(value))

	if params != nil {
		for k, v := range params {
			buf.WriteString("; ")

			var paramVal string

			switch v.(type) {
			case string:
				paramVal = fmt.Sprintf(`"%s"`, getQEncodeString(v.(string)))
			case *string:
				paramVal = fmt.Sprintf(`"%s"`, getQEncodeString(*v.(*string)))
			case time.Time:
				paramVal = fmt.Sprintf(`"%s"`, v.(time.Time).Format(TIME_FORMAT))
			case *time.Time:
				paramVal = fmt.Sprintf(`"%s"`, v.(*time.Time).Format(TIME_FORMAT))
			default:
				paramVal = fmt.Sprintf(`%v`, v)
			}

			buf.WriteString(k)
			buf.WriteString("=")
			buf.WriteString(paramVal)
		}
	}

	// result examples:
	// - image/jpeg; name="f7fb566a3f724874bf53cdb8e3b37d7a.jpg"
	// - f7fb566a3f724874bf53cdb8e3b37d7a.jpg
	// - attachment; filename="f7fb566a3f724874bf53cdb8e3b37d7a.jpg"; size=41681; creation-date="Tue, 27 Dec 2016 12:29:43 GMT"
	// - base64

	h.set(key, buf.String())
}

func (h Header) Bytes() []byte {

	// result example:
	//
	// Content-Type: image/jpeg; name="f7fb566a3f724874bf53cdb8e3b37d7a.jpg"
	// Content-Description: f7fb566a3f724874bf53cdb8e3b37d7a.jpg
	// Content-Disposition: attachment;
	//     filename="f7fb566a3f724874bf53cdb8e3b37d7a.jpg"; size=41681;
	//     creation-date="Tue, 27 Dec 2016 12:29:43 GMT";
	//     modification-date="Tue, 27 Dec 2016 12:29:43 GMT"
	// Content-Transfer-Encoding: base64

	buf := bytes.NewBuffer(nil)
	for k, v := range h {

		buf.WriteString(k)
		buf.WriteString(": ")
		buf.WriteString(strings.Join(v, "; "))
		buf.WriteString("\r\n")
	}

	return buf.Bytes()
}

func (h Header) set(key string, value string) {
	h[key] = []string{value}
}
