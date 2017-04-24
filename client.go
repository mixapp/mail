package mail

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/smtp"
	"runtime"
	"strings"
	"sync"
	"time"
)

type SmtpClient struct {
	io.Closer

	Host        string
	Port        string
	User        string
	Password    string
	Workstation string // NTLM authentication mechanism
	From        string
	MaxLifetime time.Duration
	TLSConfig   *tls.Config

	conn       *smtp.Client
	mu         sync.RWMutex
	once       sync.Once
	createTime time.Time
}

func (c *SmtpClient) Close() error {

	c.mu.Lock()
	c.conn = nil
	c.mu.Unlock()

	return nil
}

func (c *SmtpClient) Connection() (*smtp.Client, error) {

	c.once.Do(func() {
		if c.MaxLifetime == 0 {
			c.MaxLifetime = time.Minute
		}
		if c.TLSConfig == nil {
			c.TLSConfig = &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         c.Host,
			}
		}
	})

	c.mu.RLock()
	smtpConn := c.conn

	needUpdate := smtpConn == nil || (c.createTime.Year() > 0 && time.Now().Sub(c.createTime) == c.MaxLifetime)
	c.mu.RUnlock()

	if needUpdate {
		if conn, err := c.internalInit(); err != nil {
			return nil, err
		} else {
			smtpConn = conn
		}
	}

	return smtpConn, nil
}

func (c *SmtpClient) internalInit() (*smtp.Client, error) {

	c.mu.Lock()
	defer c.mu.Unlock()

	servername := fmt.Sprintf("%s:%s", c.Host, c.Port)
	host, _, err := net.SplitHostPort(servername)
	if err != nil {
		return nil, err
	}

	IS_TLS := needTLSConnection(servername)

	conn, err := net.DialTimeout("tcp", servername, 10*time.Second)
	if err != nil {
		return nil, err
	}

	if IS_TLS {
		conn = tls.Client(conn, c.TLSConfig)
	}

	cl, err := smtp.NewClient(conn, host)
	if err != nil {
		return nil, err
	}

	if !IS_TLS {
		if ok, _ := cl.Extension("STARTTLS"); ok {
			if err := cl.StartTLS(c.TLSConfig); err != nil {
				cl.Close()
				return nil, err
			}
		}
	}

	if len(c.User) > 0 {
		if ok, auths := cl.Extension("AUTH"); ok {
			var auth smtp.Auth

			if strings.Contains(auths, "CRAM-MD5") {
				auth = smtp.CRAMMD5Auth(c.User, c.Password)
			} else if strings.Contains(auths, "NTLM") {
				auth = nil
				a, err := NewNTLMAuth(host, c.User, c.Password, c.Workstation)

				if err != nil {
					cl.Close()
					return nil, err
				} else if err := SmtpNTLMAuthenticate(cl, a); err != nil {
					cl.Close()
					return nil, err
				}

			} else {
				auth = smtp.PlainAuth("", c.User, c.Password, host)
			}

			if auth != nil {
				if err := cl.Auth(auth); err != nil {
					cl.Close()
					return nil, err
				}
			}
		}
	}

	c.conn = cl
	c.createTime = time.Now()

	runtime.SetFinalizer(c.conn, func(conn *smtp.Client) {
		if err := conn.Close(); err != nil {
			log.Println("Failed close SMTP connection:", err)
		}
	})

	return c.conn, nil
}

func needTLSConnection(address string) bool {

	tlsconfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         address,
	}

	conn, err := net.DialTimeout("tcp", address, 10*time.Second)
	if err != nil {
		return false
	}

	conn = tls.Client(conn, tlsconfig)
	defer conn.Close()

	fmt.Fprintf(conn, "GET / HTTP/1.0\r\n\r\n")
	_, err = bufio.NewReader(conn).ReadString('\n')

	return err == nil
}
