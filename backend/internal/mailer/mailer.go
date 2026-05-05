package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Config 保存邮件发送配置。
type Config struct {
	Delivery  string
	Host      string
	Port      int
	Username  string
	Password  string
	From      string
	FromName  string
	UseTLS    bool
	StartTLS  bool
	Timeout   time.Duration
	AppName   string
	AppEnv    string
	DebugCode bool
}

// Client 负责向真实邮箱发送验证码。
type Client struct {
	cfg Config
}

// New 创建邮件发送客户端。
func New(cfg Config) *Client {
	if cfg.Port <= 0 {
		cfg.Port = 587
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 10 * time.Second
	}
	if strings.TrimSpace(cfg.AppName) == "" {
		cfg.AppName = "AI 生图 SaaS"
	}
	return &Client{cfg: cfg}
}

// Configured 表示当前是否已经配置真实 SMTP 发送能力。
func (c *Client) Configured() bool {
	if c == nil {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(c.cfg.Delivery), "smtp") {
		return false
	}
	return strings.TrimSpace(c.cfg.Host) != "" && strings.TrimSpace(c.cfg.From) != ""
}

// SendVerificationCode 发送邮箱验证码。
func (c *Client) SendVerificationCode(ctx context.Context, to string, code string, scene string, ttl time.Duration) error {
	if !c.Configured() {
		return fmt.Errorf("smtp mailer is not configured")
	}
	to = strings.TrimSpace(to)
	code = strings.TrimSpace(code)
	if to == "" || code == "" {
		return fmt.Errorf("email or code is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	subject := c.cfg.AppName + " 验证码"
	body := fmt.Sprintf("你的验证码是：%s\n\n验证码有效期为 %d 分钟，请不要把验证码透露给他人。\n", code, int(ttl.Minutes()))
	if strings.EqualFold(scene, "reset_password") {
		body = fmt.Sprintf("你正在重置密码，验证码是：%s\n\n验证码有效期为 %d 分钟，请不要把验证码透露给他人。\n", code, int(ttl.Minutes()))
	}

	message := buildMessage(c.formatFrom(), to, subject, body)
	return c.send(ctx, to, []byte(message))
}

func (c *Client) send(ctx context.Context, to string, message []byte) error {
	address := net.JoinHostPort(strings.TrimSpace(c.cfg.Host), fmt.Sprintf("%d", c.cfg.Port))
	dialer := net.Dialer{Timeout: c.cfg.Timeout}

	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("connect smtp server: %w", err)
	}
	defer conn.Close()

	var smtpClient *smtp.Client
	if c.cfg.UseTLS {
		tlsConn := tls.Client(conn, &tls.Config{
			ServerName:         strings.TrimSpace(c.cfg.Host),
			MinVersion:         tls.VersionTLS12,
			InsecureSkipVerify: false,
		})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			return fmt.Errorf("smtp tls handshake: %w", err)
		}
		smtpClient, err = smtp.NewClient(tlsConn, strings.TrimSpace(c.cfg.Host))
	} else {
		smtpClient, err = smtp.NewClient(conn, strings.TrimSpace(c.cfg.Host))
	}
	if err != nil {
		return fmt.Errorf("create smtp client: %w", err)
	}
	defer smtpClient.Close()

	if c.cfg.StartTLS && !c.cfg.UseTLS {
		if ok, _ := smtpClient.Extension("STARTTLS"); ok {
			if err := smtpClient.StartTLS(&tls.Config{
				ServerName: strings.TrimSpace(c.cfg.Host),
				MinVersion: tls.VersionTLS12,
			}); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
	}

	username := strings.TrimSpace(c.cfg.Username)
	password := strings.TrimSpace(c.cfg.Password)
	if username != "" || password != "" {
		auth := smtp.PlainAuth("", username, password, strings.TrimSpace(c.cfg.Host))
		if err := smtpClient.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := smtpClient.Mail(strings.TrimSpace(c.cfg.From)); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := smtpClient.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	writer, err := smtpClient.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := writer.Write(message); err != nil {
		_ = writer.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	if err := smtpClient.Quit(); err != nil {
		return fmt.Errorf("smtp quit: %w", err)
	}
	return nil
}

func (c *Client) formatFrom() string {
	from := strings.TrimSpace(c.cfg.From)
	name := strings.TrimSpace(c.cfg.FromName)
	if name == "" {
		name = strings.TrimSpace(c.cfg.AppName)
	}
	if name == "" {
		return from
	}
	return fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", name), from)
}

func buildMessage(from string, to string, subject string, body string) string {
	headers := []string{
		"From: " + from,
		"To: " + strings.TrimSpace(to),
		"Subject: " + mime.QEncoding.Encode("utf-8", subject),
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"Content-Transfer-Encoding: 8bit",
	}
	return strings.Join(headers, "\r\n") + "\r\n\r\n" + body
}
