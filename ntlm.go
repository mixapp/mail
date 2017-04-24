package mail

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"net/smtp"
	"net/textproto"
	"strings"

	mssql "github.com/denisenkom/go-mssqldb"
)

// NTLMAuth implements smtp.Auth. The authentication mechanism.
type NTLMAuth struct {
	mssql.NTLMAuth
	Host string
}

func NewNTLMAuth(host, user, password, workstation string) (*NTLMAuth, error) {

	domanAndUsername := strings.SplitN(user, `\`, 2)
	if len(domanAndUsername) != 2 {
		return nil, errors.New(`Wrong format of username. The required format is 'domain\username'`)
	}

	a := mssql.NTLMAuth{
		Domain:      domanAndUsername[0],
		UserName:    domanAndUsername[1],
		Password:    password,
		Workstation: workstation,
	}

	n := NTLMAuth{
		NTLMAuth: a,
		Host:     host,
	}

	return &n, nil
}

func (n *NTLMAuth) Start(server *smtp.ServerInfo) (string, []byte, error) {

	if !server.TLS {
		var isNTLM bool
		for _, mechanism := range server.Auth {
			isNTLM = isNTLM || mechanism == "NTLM"
		}

		if !isNTLM {
			return "", nil, errors.New("mail: unknown authentication type:" + fmt.Sprintln(server.Auth))
		}
	}

	if server.Name != n.Host {
		return "", nil, errors.New("mail: wrong host name")
	}

	return "NTLM", nil, nil
}

func (n *NTLMAuth) Next(fromServer []byte, more bool) ([]byte, error) {

	if !more {
		return nil, nil
	}

	switch {
	case bytes.Equal(fromServer, []byte("NTLM supported")):
		return n.InitialBytes()
	default:
		maxLen := base64.StdEncoding.DecodedLen(len(fromServer))

		dst := make([]byte, maxLen)
		resultLen, err := base64.StdEncoding.Decode(dst, fromServer)
		if err != nil {
			return nil, errors.New(fmt.Sprintf("Decode base64 error: %s", err.Error()))
		}

		var challengeMessage []byte
		if maxLen == resultLen {
			challengeMessage = dst
		} else {
			challengeMessage = make([]byte, resultLen, resultLen)
			copy(challengeMessage, dst)
		}

		return n.NextBytes(challengeMessage)
	}
}

// Copy-paste from smtp.Client.Auth for minor modified
func SmtpNTLMAuthenticate(c *smtp.Client, a *NTLMAuth) error {

	_, tls := c.TLSConnectionState()
	_, auth := c.Extension("AUTH")

	encoding := base64.StdEncoding
	mech, resp, err := a.Start(&smtp.ServerInfo{a.Host, tls, strings.SplitN(auth, " ", -1)})

	if err != nil {
		c.Quit()
		return err
	}
	resp64 := make([]byte, encoding.EncodedLen(len(resp)))
	encoding.Encode(resp64, resp)
	code, msg64, err := cmd(c, 0, strings.TrimSpace(fmt.Sprintf("AUTH %s %s", mech, resp64)))
	for err == nil {
		var msg []byte

		switch code {
		case 334:
			// Modified that moment (https://msdn.microsoft.com/en-us/library/windows/desktop/aa378749(v=vs.85).aspx)
			// Source code:
			// 	msg, err = encoding.DecodeString(msg64)
			// New code:
			msg = []byte(msg64)
		case 235:
			// the last message isn't base64 because it isn't a challenge
			msg = []byte(msg64)
		default:
			err = &textproto.Error{Code: code, Msg: msg64}
		}

		if err == nil {
			resp, err = a.Next(msg, code == 334)
		}
		if err != nil {
			// abort the AUTH
			cmd(c, 501, "*")
			c.Quit()
			break
		}
		if resp == nil {
			break
		}

		resp64 = make([]byte, encoding.EncodedLen(len(resp)))
		encoding.Encode(resp64, resp)
		code, msg64, err = cmd(c, 0, string(resp64))
	}
	return err
}

// Copy-paste because is private method of smtp.Client
func cmd(c *smtp.Client, expectCode int, format string, args ...interface{}) (int, string, error) {
	id, err := c.Text.Cmd(format, args...)
	if err != nil {
		return 0, "", err
	}
	c.Text.StartResponse(id)
	defer c.Text.EndResponse(id)
	code, msg, err := c.Text.ReadResponse(expectCode)
	return code, msg, err
}
