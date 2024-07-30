package mailreader

import (
	"context"
	"errors"
	"time"

	"github.com/New-Moon-Team/gomailreader/proxy"
)

type Reader interface {
	BoxGetAll(box MailBox, res *[]byte) error
	GetAllBoxes() error
	GetLatestMsgOf(ctx context.Context, res *[]byte, box, receiver string) error
	log(l string)
	warn(w string)
}

type ReaderConfig struct {
	Server   ServerMail
	User     string
	Password string
	Proxy    *proxy.Config
}

var (
	ErrInvalidReaderType        = errors.New("invalid reader type")
	ErrServerMailNotImplemented = errors.New("mail server not implemented")
	ErrNoProxy                  = errors.New("no proxy")
	ErrInvalidBox               = errors.New("invalid box")
	ErrInvalidResponse          = errors.New("invalid response")
	ErrNoLogger                 = errors.New("no logger")
)

type ReaderType string

var (
	ReaderTypeImap ReaderType = "imap"
	ReaderTypePop3 ReaderType = "pop3"
)

type ServerMail string

var (
	ImapGmailServer   ServerMail = "imap.gmail.com"
	ImapHotmailServer ServerMail = "imap-mail.outlook.com"
)

type Pop3Server string

var (
	Pop3GmailServer   ServerMail = "pop.gmail.com"
	Pop3HotmailServer ServerMail = "pop-mail.outlook.com"
)

type MailBox string

const (
	ImapGmailInbox   MailBox = "INBOX"
	ImapGmailSpam    MailBox = "[Gmail]/Spam"
	ImapHotmailInbox MailBox = "INBOX"
	ImapHotmailSpam  MailBox = "Junk"
	Pop3DefaultBox   MailBox = "Inbox"
)

type ImapMailPart struct {
	ContentType string
	Content     string
}
type Pop3MailPart struct {
	ContentType string
	Content     string
}
type ImapMail struct {
	Uid uint32 `json:"uid"`
	// The message date.
	Date time.Time `json:"date"`
	// The message subject.
	Subject string `json:"subject"`
	// The From header addresses.
	From []string `json:"from"`
	// The message senders.
	Sender []string `json:"sender"`
	// The Reply-To header addresses.
	ReplyTo []string `json:"reply_to"`
	// The To header addresses.
	To []string `json:"to"`
	// The Cc header addresses.
	Cc []string `json:"cc"`
	// The Bcc header addresses.
	Bcc []string `json:"bcc"`
	// The In-Reply-To header. Contains the parent Message-Id.
	InReplyTo string `json:"in_reply_to"`
	// The Message-Id header.
	MessageId  string         `json:"message_id"`
	Parts      []ImapMailPart `json:"parts"`
	Box        string         `json:"box"`
	ScantAt    time.Time      `json:"scant_at"`
	ScanMethod string         `json:"scan_method"`
	Email      string         `json:"email"`
}
type Pop3Mail struct {
	Uid uint32 `json:"uid"`
	// The message date.
	Date string `json:"date"`
	// The message subject.
	Subject string `json:"subject"`
	// The From header addresses.
	From string `json:"from"`
	// The message senders.
	Sender string `json:"sender"`
	// The Reply-To header addresses.
	ReplyTo string `json:"reply_to"`
	// The To header addresses.
	To string `json:"to"`
	// The Cc header addresses.
	Cc string `json:"cc"`
	// The Bcc header addresses.
	Bcc string `json:"bcc"`
	// The In-Reply-To header. Contains the parent Message-Id.
	InReplyTo string `json:"in_reply_to"`
	// The Message-Id header.
	MessageId  string         `json:"message_id"`
	Parts      []Pop3MailPart `json:"parts"`
	Box        string         `json:"box"`
	ScantAt    time.Time      `json:"scant_at"`
	ScanMethod string         `json:"scan_method"`
	Email      string         `json:"email"`
}

func GetReader(t ReaderType, cfg *ReaderConfig) (Reader, error) {
	switch t {
	case ReaderTypeImap:
		r := &ImapReader{*cfg}
		return r, nil
	case ReaderTypePop3:
		r := &Pop3Reader{*cfg}
		return r, nil
	}

	return nil, ErrInvalidReaderType
}
