package mail

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// SMTPPortImplicitTLS 是隐式 TLS（SMTPS）端口，走先建 TLS 连接再握手 SMTP 的方式。
const SMTPPortImplicitTLS = 465

type SMTPConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

// Configured 判断 SMTP 是否已配置齐全，未配置时上层应关闭邮件验证码功能。
func (config SMTPConfig) Configured() bool {
	return config.Host != "" && config.Username != "" && config.Password != ""
}

type SMTPSender struct {
	config SMTPConfig
}

func NewSMTPSender(config SMTPConfig) *SMTPSender {
	if config.From == "" {
		config.From = config.Username
	}
	return &SMTPSender{config: config}
}

// SendLoginCode 发送登录验证码邮件，正文为纯文本中文内容。
func (sender *SMTPSender) SendLoginCode(to string, code string, ttl time.Duration) error {
	config := sender.config
	if !config.Configured() {
		return fmt.Errorf("smtp is not configured")
	}

	subject := "Rendering CMS 登录验证码"
	body := strings.Join([]string{
		"你正在登录 Rendering CMS 后台。",
		"",
		"验证码：" + code,
		"",
		"验证码 " + strconv.Itoa(int(ttl.Minutes())) + " 分钟内有效，且只能使用一次。",
		"如果这不是你本人的操作，请立即修改账号密码。",
	}, "\r\n")

	message := strings.Join([]string{
		"From: " + config.From,
		"To: " + to,
		"Subject: " + subject,
		"Date: " + time.Now().Format(time.RFC1123Z),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
		"",
		body,
	}, "\r\n")

	addr := config.Host + ":" + strconv.Itoa(config.Port)
	auth := smtp.PlainAuth("", config.Username, config.Password, config.Host)

	if config.Port == SMTPPortImplicitTLS {
		return sender.sendImplicitTLS(addr, config.Host, auth, config.From, to, message)
	}
	return smtp.SendMail(addr, auth, config.From, []string{to}, []byte(message))
}

func (sender *SMTPSender) sendImplicitTLS(addr string, host string, auth smtp.Auth, from string, to string, message string) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return fmt.Errorf("dial smtp over tls: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}
	defer client.Close()

	if err = client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err = client.Mail(from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err = writer.Write([]byte(message)); err != nil {
		return fmt.Errorf("smtp write body: %w", err)
	}
	if err = writer.Close(); err != nil {
		return fmt.Errorf("smtp close body: %w", err)
	}
	return client.Quit()
}
