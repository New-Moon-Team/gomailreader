package mailreader

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mailreader/proxy"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

type ImapReader struct {
	ReaderConfig
}

func (r *ImapReader) BoxGetAll(mailbox MailBox, res *[]byte) error {
	box := fmt.Sprintf("%v", mailbox)

	switch r.Server {
	case ImapGmailServer:
		mails, err := r.gmailBoxGetAll(box)
		if err != nil {
			return err
		}
		b, err := json.Marshal(mails)
		if err != nil {
			return err
		}
		*res = b
	case ImapHotmailServer:
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
func (r *ImapReader) GetAllBoxes() error {
	//TODO: implement further on demand
	d := proxy.Direct

	addr := fmt.Sprintf("%v:%s", r.Server, "993")

	r.log(fmt.Sprintf("Dialing address %v", addr))
	c, err := client.DialWithDialerTLS(d, addr, &tls.Config{InsecureSkipVerify: true})
	//c, err := client.DialWithDialerTLS(d, "imap-mail.outlook.com:993", &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return err
	}
	defer c.Logout()

	err = c.Login(r.User, r.Password)
	if err != nil {
		return err
	}

	// List mailboxes
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", mailboxes)
	}()

	log.Println("Mailboxes:")
	for m := range mailboxes {
		log.Println("* " + m.Name)
	}

	return nil
}
func (r *ImapReader) GetLatestMsgOf(ctx context.Context, res *[]byte, box, receiver string) error {
	d := proxy.Direct

	addr := fmt.Sprintf("%v:%s", r.Server, "993")

	r.log(fmt.Sprintf("Dialing address %v", addr))
	c, err := client.DialWithDialerTLS(d, addr, &tls.Config{InsecureSkipVerify: true})
	//c, err := client.DialWithDialerTLS(d, "imap-mail.outlook.com:993", &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return err
	}
	defer c.Logout()

	err = c.Login(r.User, r.Password)
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			_, err = c.Select(box, false)
			if err != nil {
				return err
			}

			cr := imap.NewSearchCriteria()
			//since last 5 minutes
			cr.Since = time.Now().Add(-5 * time.Minute)
			cr.WithoutFlags = []string{imap.SeenFlag}
			ids, err := c.Search(cr)
			if err != nil {
				return err
			}
			fmt.Printf("Search result: %v\n", ids)

			if len(ids) == 0 {
				time.Sleep(time.Second * 5)
				continue
			}

			seqset := new(imap.SeqSet)
			seqset.AddNum(ids...)

			messages := make(chan *imap.Message, 10)
			done := make(chan error, 1)
			go func() {
				done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
			}()

			var (
				seqn  uint32
				mtime time.Time
			)
			for msg := range messages {
				for _, t := range msg.Envelope.To {
					// fmt.Printf("to: %s\n", t.Address())
					if t.Address() == receiver {
						if seqn == 0 || mtime.Before(msg.Envelope.Date) {
							seqn = msg.SeqNum
							mtime = msg.Envelope.Date
							fmt.Printf("mid: %d\n", seqn)
						}
					}
				}
			}
			if err := <-done; err != nil {
				return err
			}

			if seqn == 0 {
				time.Sleep(time.Second * 5)
				continue
			}

			seqset = new(imap.SeqSet)
			seqset.AddNum(seqn)

			messages = make(chan *imap.Message, 1)
			done = make(chan error, 1)
			go func() {
				done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchUid, imap.FetchEnvelope, imap.FetchRFC822}, messages)
			}()

			msg := <-messages
			m, err := r.parseMsg(msg)
			if err != nil {
				log.Printf("parse msg error: %v\n", err)
				time.Sleep(time.Second * 5)
				continue
			}

			b, err := json.Marshal(m)
			if err != nil {
				return err
			}
			*res = b

			defer func() {
				item := imap.FormatFlagsOp(imap.AddFlags, true)
				flags := []interface{}{imap.SeenFlag}
				err = c.Store(seqset, item, flags, nil)
				if err != nil {
					log.Printf("err mark mail seen: %v\n", err)
				}
			}()

			return nil
		}
	}
	return nil
}

func (r *ImapReader) parseMsg(msg *imap.Message) (*ImapMail, error) {
	var ml ImapMail

	ml.Uid = msg.Uid

	for _, value := range msg.Body {
		m, err := mail.ReadMessage(value)
		if err != nil {
			r.warn(fmt.Sprintf("Warn: reading message error %v", err))
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

				var mp ImapMailPart
				mp.ContentType = p.Header.Get("Content-Type")
				mp.Content = string(slurp)

				ml.Parts = append(ml.Parts, mp)
			}
		}
	}

	ml.Date = msg.Envelope.Date.Local()
	ml.Subject = msg.Envelope.Subject
	for _, a := range msg.Envelope.From {
		ml.From = append(ml.From, a.Address())
	}
	for _, a := range msg.Envelope.Sender {
		ml.Sender = append(ml.Sender, a.Address())
	}
	for _, a := range msg.Envelope.ReplyTo {
		ml.ReplyTo = append(ml.ReplyTo, a.Address())
	}
	for _, a := range msg.Envelope.To {
		ml.To = append(ml.To, a.Address())
	}
	for _, a := range msg.Envelope.Cc {
		ml.Cc = append(ml.Cc, a.Address())
	}
	for _, a := range msg.Envelope.Bcc {
		ml.Bcc = append(ml.Bcc, a.Address())
	}
	ml.InReplyTo = msg.Envelope.InReplyTo
	ml.MessageId = msg.Envelope.MessageId

	return &ml, nil
}

func (r *ImapReader) gmailBoxGetAll(box string) ([]ImapMail, error) {
	r.log(fmt.Sprintf("Start reading box: %v", box))

	var mails []ImapMail

	d := proxy.Direct

	addr := fmt.Sprintf("%v:%s", r.Server, "993")

	r.log(fmt.Sprintf("Dialing address %v", addr))
	c, err := client.DialWithDialerTLS(d, addr, &tls.Config{InsecureSkipVerify: true})
	//c, err := client.DialWithDialerTLS(d, "imap-mail.outlook.com:993", &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return mails, err
	}
	defer c.Logout()

	err = c.Login(r.User, r.Password)
	if err != nil {
		return mails, err
	}
	r.log(fmt.Sprintf("Logged in as %v", r.User))

	r.log(fmt.Sprintf("Selecting box: %v", box))
	mbox, err := c.Select(box, false)
	if err != nil {
		return mails, err
	}
	r.log(fmt.Sprintf("Message count: %d", mbox.Messages))

	if mbox.Messages == 0 {
		return mails, nil
	}

	seqset := new(imap.SeqSet)
	seqset.AddRange(1, mbox.Messages)

	messages := make(chan *imap.Message)
	done := make(chan error, 1)

	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchUid, imap.FetchEnvelope, imap.FetchRFC822}, messages)
	}()

	r.log("Converting messages")
	for msg := range messages {
		var ml ImapMail

		ml.Uid = msg.Uid

		for _, value := range msg.Body {
			m, err := mail.ReadMessage(value)
			if err != nil {
				r.warn(fmt.Sprintf("Warn: reading message error %v", err))
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

					var mp ImapMailPart
					mp.ContentType = p.Header.Get("Content-Type")
					mp.Content = string(slurp)

					ml.Parts = append(ml.Parts, mp)
				}
			}
		}

		ml.Date = msg.Envelope.Date
		ml.Subject = msg.Envelope.Subject
		for _, a := range msg.Envelope.From {
			ml.From = append(ml.From, a.Address())
		}
		for _, a := range msg.Envelope.Sender {
			ml.Sender = append(ml.Sender, a.Address())
		}
		for _, a := range msg.Envelope.ReplyTo {
			ml.ReplyTo = append(ml.ReplyTo, a.Address())
		}
		for _, a := range msg.Envelope.To {
			ml.To = append(ml.To, a.Address())
		}
		for _, a := range msg.Envelope.Cc {
			ml.Cc = append(ml.Cc, a.Address())
		}
		for _, a := range msg.Envelope.Bcc {
			ml.Bcc = append(ml.Bcc, a.Address())
		}
		ml.InReplyTo = msg.Envelope.InReplyTo
		ml.MessageId = msg.Envelope.MessageId
		ml.Box = box
		ml.ScanMethod = "IMAP"
		ml.ScantAt = time.Now()
		ml.Email = r.User

		mails = append(mails, ml)
	}

	if err := <-done; err != nil {
		r.warn(fmt.Sprintf("Warn: reading box error %v", err))
		return mails, err
	} else {
		r.log("Reading box completed")
	}

	return mails, nil
}
func (r *ImapReader) hotmailBoxGetAll(box string) ([]ImapMail, error) {
	r.log(fmt.Sprintf("Start reading box: %v", box))

	var mails []ImapMail

	d := proxy.Direct

	addr := fmt.Sprintf("%v:%s", r.Server, "993")

	r.log(fmt.Sprintf("Dialing address %v", addr))
	c, err := client.DialWithDialerTLS(d, addr, &tls.Config{InsecureSkipVerify: true})
	//c, err := client.DialWithDialerTLS(d, "imap-mail.outlook.com:993", &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return mails, err
	}
	defer c.Logout()

	err = c.Login(r.User, r.Password)
	if err != nil {
		return mails, err
	}
	r.log(fmt.Sprintf("Logged in as %v", r.User))

	r.log(fmt.Sprintf("Selecting box: %v", box))
	mbox, err := c.Select(box, false)
	if err != nil {
		return mails, err
	}
	r.log(fmt.Sprintf("Message count: %d", mbox.Messages))

	if mbox.Messages == 0 {
		return mails, nil
	}

	seqset := new(imap.SeqSet)
	seqset.AddRange(1, mbox.Messages)

	messages := make(chan *imap.Message)
	done := make(chan error, 1)

	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchUid, imap.FetchEnvelope, imap.FetchRFC822}, messages)
	}()

	r.log("Converting messages")
	for msg := range messages {
		var ml ImapMail

		ml.Uid = msg.Uid

		for _, value := range msg.Body {
			m, err := mail.ReadMessage(value)
			if err != nil {
				r.warn(fmt.Sprintf("Warn: reading message error %v", err))
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

					var mp ImapMailPart
					mp.ContentType = p.Header.Get("Content-Type")
					mp.Content = string(slurp)

					ml.Parts = append(ml.Parts, mp)
				}
			}
		}

		ml.Date = msg.Envelope.Date
		ml.Subject = msg.Envelope.Subject
		for _, a := range msg.Envelope.From {
			ml.From = append(ml.From, a.Address())
		}
		for _, a := range msg.Envelope.Sender {
			ml.Sender = append(ml.Sender, a.Address())
		}
		for _, a := range msg.Envelope.ReplyTo {
			ml.ReplyTo = append(ml.ReplyTo, a.Address())
		}
		for _, a := range msg.Envelope.To {
			ml.To = append(ml.To, a.Address())
		}
		for _, a := range msg.Envelope.Cc {
			ml.Cc = append(ml.Cc, a.Address())
		}
		for _, a := range msg.Envelope.Bcc {
			ml.Bcc = append(ml.Bcc, a.Address())
		}
		ml.InReplyTo = msg.Envelope.InReplyTo
		ml.MessageId = msg.Envelope.MessageId
		ml.Box = box
		ml.ScanMethod = "IMAP"
		ml.ScantAt = time.Now()
		ml.Email = r.User

		mails = append(mails, ml)
	}

	if err := <-done; err != nil {
		return mails, err
	}

	return mails, nil
}

func (r *ImapReader) log(l string) {
}
func (r *ImapReader) warn(w string) {
}
