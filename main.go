package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"net/mail"
	"strings"
	"time"

	"github.com/alash3al/go-smtpsrv"
	"github.com/go-resty/resty/v2"
)

func main() {
	cfg := smtpsrv.ServerConfig{
		ReadTimeout:     time.Duration(*flagReadTimeout) * time.Second,
		WriteTimeout:    time.Duration(*flagWriteTimeout) * time.Second,
		ListenAddr:      *flagListenAddr,
		MaxMessageBytes: int(*flagMaxMessageSize),
		BannerDomain:    *flagServerName,
		Handler: smtpsrv.HandlerFunc(func(c *smtpsrv.Context) error {
			msg, err := c.Parse()
			if err != nil {
				return errors.New("Cannot read your message: " + err.Error())
			}

			spfResult, _, _ := c.SPF()

			jsonData := EmailMessage{
				ID:            msg.MessageID,
				Date:          msg.Date.String(),
				References:    msg.References,
				SPFResult:     spfResult.String(),
				ResentDate:    msg.ResentDate.String(),
				ResentID:      msg.ResentMessageID,
				Subject:       msg.Subject,
				Attachments:   []*EmailAttachment{},
				EmbeddedFiles: []*EmailEmbeddedFile{},
			}

			jsonData.Body.HTML = string(msg.HTMLBody)
			jsonData.Body.Text = string(msg.TextBody)

			jsonData.Addresses.From = transformStdAddressToEmailAddress([]*mail.Address{c.From()})[0]
			jsonData.Addresses.To = transformStdAddressToEmailAddress([]*mail.Address{c.To()})[0]

			toSplited := strings.Split(jsonData.Addresses.To.Address, "@")
			if len(*flagDomain) > 0 && (len(toSplited) < 2 || toSplited[1] != *flagDomain) {
				log.Println("domain not allowed")
				log.Println(*flagDomain)
				return errors.New("Unauthorized TO domain")
			}

			jsonData.Addresses.Cc = transformStdAddressToEmailAddress(msg.Cc)
			jsonData.Addresses.Bcc = transformStdAddressToEmailAddress(msg.Bcc)
			jsonData.Addresses.ReplyTo = transformStdAddressToEmailAddress(msg.ReplyTo)
			jsonData.Addresses.InReplyTo = msg.InReplyTo

			if resentFrom := transformStdAddressToEmailAddress(msg.ResentFrom); len(resentFrom) > 0 {
				jsonData.Addresses.ResentFrom = resentFrom[0]
			}

			jsonData.Addresses.ResentTo = transformStdAddressToEmailAddress(msg.ResentTo)
			jsonData.Addresses.ResentCc = transformStdAddressToEmailAddress(msg.ResentCc)
			jsonData.Addresses.ResentBcc = transformStdAddressToEmailAddress(msg.ResentBcc)

			for _, a := range msg.Attachments {
				data, _ := io.ReadAll(a.Data)
				jsonData.Attachments = append(jsonData.Attachments, &EmailAttachment{
					Filename:    a.Filename,
					ContentType: a.ContentType,
					Data:        base64.StdEncoding.EncodeToString(data),
				})
			}

			for _, a := range msg.EmbeddedFiles {
				data, _ := io.ReadAll(a.Data)
				jsonData.EmbeddedFiles = append(jsonData.EmbeddedFiles, &EmailEmbeddedFile{
					CID:         a.CID,
					ContentType: a.ContentType,
					Data:        base64.StdEncoding.EncodeToString(data),
				})
			}

			// content, err := json.Marshal(jsonData)
			content := ""
			timestamp, err := time.Parse("2006-01-02 15:04:05 -0700 -0700", jsonData.Date)

			mainEmbed := DiscordEmbed{
				Title:       jsonData.Subject,
				Description: jsonData.Body.Text,
				Footer: DiscordFooter{
					Text: "",
				},
				Color:     8061094,
				Timestamp: timestamp.Format(time.RFC3339),
			}

			packet := DiscordMessage{
				Content:  string(content),
				Embeds:   []DiscordEmbed{mainEmbed},
				Username: jsonData.Addresses.From.Address,
			}

			resp, err := resty.New().R().SetHeader("Content-Type", "application/json").SetBody(packet).Post(*flagWebhook)
			if err != nil {
				log.Println(err)
				return errors.New("E1: Cannot accept your message due to internal error, please report that to our engineers")
			} else if resp.StatusCode() != 204 {
				log.Println(resp.Status())
				return errors.New("E2: Cannot accept your message due to internal error, please report that to our engineers")
			}

			return nil
		}),
	}

	fmt.Println(smtpsrv.ListenAndServe(&cfg))
}
