package mail

import (
	"log"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestClient(t *testing.T) {

	log.SetFlags(log.Lshortfile)
	cl := getSmtpClient()
	cl.MaxLifetime = time.Second / 2

	wg := new(sync.WaitGroup)

	for thread := 0; thread < 10; thread++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for i := 0; i < 1000; i++ {
				if i%2 == 0 {
					runtime.GC()
				} else if _, err := cl.Connection(); err != nil {
					t.Fatal(err)
				}

			}
		}()
	}

	wg.Wait()

	cl.Close()
	runtime.GC()
	time.Sleep(time.Second * 3)
}

// You can use that 'https://hub.docker.com/r/velaluqa/iredmail/' mail server for tests

func getSmtpClient() *SmtpClient {
	cl := SmtpClient{
		Host:     "localhost",
		Port:     "587",
		User:     "postmaster@example.org",
		Password: "teivVedJin",
		From:     "postmaster@example.org",
	}

	return &cl
}
