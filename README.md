# Mail Reader

This package provides a set of tools for reading emails from different mail servers using either IMAP or POP3 protocols.

## Installation

To install this package, you need to have Go installed on your machine. Once you have Go installed, you can clone this repository and run `go build`.

## Usage

This package provides two main types of readers: `ImapReader` and `Pop3Reader`. Both of these readers implement the `Reader` interface defined in [reader.go](reader.go).

Here's a basic example of how to use the `ImapReader`:

```go
config := mailreader.ReaderConfig{
    Server:   mailreader.ImapGmailServer,
    User:     "your-email@gmail.com",
    Password: "your-password",
}

reader := mailreader.ImapReader{ReaderConfig: config}

var res []byte
err := reader.BoxGetAll(mailreader.ImapGmailInbox, &res)
if err != nil {
    log.Fatal(err)
}

fmt.Println(string(res))
```

And here's an example of how to use the `Pop3Reader`:

```go
config := mailreader.ReaderConfig{
    Server:   mailreader.Pop3GmailServer,
    User:     "your-email@gmail.com",
    Password: "your-password",
}

reader := mailreader.Pop3Reader{ReaderConfig: config}

var res []byte
err := reader.BoxGetAll(mailreader.Pop3DefaultBox, &res)
if err != nil {
    log.Fatal(err)
}

fmt.Println(string(res))
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License.