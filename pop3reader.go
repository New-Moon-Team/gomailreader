package mailreader

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strings"
	"time"

	"github.com/New-Moon-Team/gomailreader/proxy"

	"github.com/denisss025/go-pop3-client"
)

type Pop3Reader struct {
	ReaderConfig
}

func (r *Pop3Reader) BoxGetAll(mailbox MailBox, res *[]byte) error {
	box := fmt.Sprintf("%v", mailbox)

	switch r.Server {
	case Pop3GmailServer:
		mails, err := r.gmailBoxGetAll(box)
		if err != nil {
			return err
		}
		b, err := json.Marshal(mails)
		if err != nil {
			return err
		}
		*res = b
	case Pop3HotmailServer:
		mails, err := r.hotmailBoxGetAll(box)
		if err != nil {
			return err
		}
		b, err := json.Marshal(mails)
		if err != nil {
			return err
		}
		*res = b
	default:
		return ErrServerMailNotImplemented
	}

	return nil
}
func (r *Pop3Reader) GetAllBoxes() error {
	fmt.Println("it works")
	return nil
}
func (r *Pop3Reader) GetLatestMsgOf(ctx context.Context, res *[]byte, box, receiver string) error {
	panic("implement me")
}

func (r *Pop3Reader) gmailBoxGetAll(box string) ([]Pop3Mail, error) {
	r.log(fmt.Sprintf("Start reading box: %v", box))

	var mails []Pop3Mail

	if r.Proxy == nil {
		return mails, ErrNoProxy
	}

	d, err := proxy.NewHTTPDialer(r.Proxy)
	if err != nil {
		return mails, err
	}

	addr := fmt.Sprintf("%v:%v", r.Server, 995)

	r.log(fmt.Sprintf("Dialing address %v", addr))
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		return mails, err
	}
	tlsConn := tls.Client(conn, &tls.Config{InsecureSkipVerify: true})

	c, err := pop3.NewClient(tlsConn)
	if err != nil {
		return mails, err
	}
	defer c.Quit()
	defer c.Close()

	err = c.Login(r.User, r.Password)
	if err != nil {
		return mails, err
	}
	r.log(fmt.Sprintf("Logged in as %v", r.User))

	messages, err := c.GetMessages()
	if err != nil {
		return mails, err
	}
	r.log(fmt.Sprintf("Message count: %d", len(messages)))

	r.log("Converting messages")
	for _, msg := range messages {
		var ml Pop3Mail

		_, m, err := msg.Retrieve()
		if err != nil {
			r.warn(fmt.Sprintf("Warn: error retriving mail %v", err))
			continue
		}

		mediaType, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
		if err != nil {
			r.warn(fmt.Sprintf("Warn: parsing media type error %v", err))
			continue
		}

		if strings.HasPrefix(mediaType, "multipart/") {
			mr := multipart.NewReader(m.Body, params["boundary"])

			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					break
				}

				if err != nil {
					r.warn(fmt.Sprintf("Warn: getting part error %v", err))
					continue
				}

				slurp, err := io.ReadAll(p)
				if err != nil {
					r.warn(fmt.Sprintf("Warn: reading part error %v", err))
					continue
				}

				var mp Pop3MailPart
				mp.ContentType = p.Header.Get("Content-Type")
				mp.Content = string(slurp)
				ml.ScanMethod = "POP3"
				ml.ScantAt = time.Now()

				ml.Parts = append(ml.Parts, mp)
			}
		}

		ml.Date = m.Header.Get("Date")
		ml.Subject = m.Header.Get("Subject")
		ml.From = m.Header.Get("From")
		ml.Sender = m.Header.Get("Sender")
		ml.ReplyTo = m.Header.Get("Reply-To")
		ml.To = m.Header.Get("To")
		ml.Cc = m.Header.Get("Cc")
		ml.Bcc = m.Header.Get("Bcc")
		ml.InReplyTo = m.Header.Get("In-Reply-To")
		ml.MessageId = m.Header.Get("Message-Id")
		ml.Email = r.User

		mails = append(mails, ml)
	}

	return mails, nil
}
func (r *Pop3Reader) hotmailBoxGetAll(box string) ([]Pop3Mail, error) {
	r.log(fmt.Sprintf("Start reading box: %v", box))

	var mails []Pop3Mail

	if r.Proxy == nil {
		return mails, ErrNoProxy
	}

	d, err := proxy.NewHTTPDialer(r.Proxy)
	if err != nil {
		return mails, err
	}
	fmt.Printf("start dialing server: %v\n", r.Server)

	addr := fmt.Sprintf("%v:%v", r.Server, 995)

	r.log(fmt.Sprintf("Dialing address %v", addr))
	conn, err := d.Dial("tcp", addr)
	if err != nil {
		return mails, err
	}
	tlsConn := tls.Client(conn, &tls.Config{InsecureSkipVerify: true})

	c, err := pop3.NewClient(tlsConn)
	if err != nil {
		return mails, err
	}
	defer c.Quit()
	defer c.Close()

	err = c.Login(r.User, r.Password)
	if err != nil {
		return mails, err
	}
	r.log(fmt.Sprintf("Logged in as %v", r.User))

	messages, err := c.GetMessages()
	if err != nil {
		return mails, err
	}
	r.log(fmt.Sprintf("Message count: %d", len(messages)))

	r.log("Converting messages")
	for _, msg := range messages {
		var ml Pop3Mail

		_, m, err := msg.Retrieve()
		if err != nil {
			r.warn(fmt.Sprintf("Warn: error retriving mail %v", err))
			continue
		}

		mediaType, params, err := mime.ParseMediaType(m.Header.Get("Content-Type"))
		if err != nil {
			r.warn(fmt.Sprintf("Warn: parsing media type error %v", err))
			continue
		}

		if strings.HasPrefix(mediaType, "multipart/") {
			mr := multipart.NewReader(m.Body, params["boundary"])

			for {
				p, err := mr.NextPart()
				if err == io.EOF {
					break
				}

				if err != nil {
					r.warn(fmt.Sprintf("Warn: getting part error %v", err))
					continue
				}

				slurp, err := io.ReadAll(p)
				if err != nil {
					r.warn(fmt.Sprintf("Warn: reading part error %v", err))
					continue
				}

				var mp Pop3MailPart
				mp.ContentType = p.Header.Get("Content-Type")
				mp.Content = string(slurp)
				ml.ScanMethod = "POP3"
				ml.ScantAt = time.Now()

				ml.Parts = append(ml.Parts, mp)
			}
		}

		ml.Date = m.Header.Get("Date")
		ml.Subject = m.Header.Get("Subject")
		ml.From = m.Header.Get("From")
		ml.Sender = m.Header.Get("Sender")
		ml.ReplyTo = m.Header.Get("Reply-To")
		ml.To = m.Header.Get("To")
		ml.Cc = m.Header.Get("Cc")
		ml.Bcc = m.Header.Get("Bcc")
		ml.InReplyTo = m.Header.Get("In-Reply-To")
		ml.MessageId = m.Header.Get("Message-Id")
		ml.Email = r.User

		mails = append(mails, ml)
	}

	return mails, nil
}

func (r *Pop3Reader) log(l string) {
}
func (r *Pop3Reader) warn(w string) {
}
