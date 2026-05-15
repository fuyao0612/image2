package mailer

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"mime"
	"net"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Config stores email delivery settings.
type Config struct {
	Delivery              string
	Host                  string
	Port                  int
	Username              string
	Password              string
	From                  string
	FromName              string
	UseTLS                bool
	StartTLS              bool
	Timeout               time.Duration
	ResendKey             string
	ResendURL             string
	TencentSecretID       string
	TencentSecretKey      string
	TencentToken          string
	TencentRegion         string
	TencentEndpoint       string
	TencentFrom           string
	TencentFromName       string
	TencentTemplateID     int64
	TencentTaskTemplateID int64
	AppName               string
	AppEnv                string
	DebugCode             bool
}

// Client sends verification-code emails.
type Client struct {
	cfg Config
}

type TaskNotification struct {
	TaskID     string
	Status     string
	Message    string
	RefundNote string
}

// New creates an email client.
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

// Configured reports whether a real email delivery provider is configured.
func (c *Client) Configured() bool {
	if c == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(c.cfg.Delivery)) {
	case "smtp":
		return strings.TrimSpace(c.cfg.Host) != "" && strings.TrimSpace(c.cfg.From) != ""
	case "resend":
		return strings.TrimSpace(c.resendKey()) != "" && strings.TrimSpace(c.cfg.From) != ""
	case "tencent_ses":
		return strings.TrimSpace(c.cfg.TencentSecretID) != "" &&
			strings.TrimSpace(c.cfg.TencentSecretKey) != "" &&
			strings.TrimSpace(c.tencentFrom()) != "" &&
			(c.cfg.TencentTemplateID > 0 || c.cfg.TencentTaskTemplateID > 0)
	default:
		return false
	}
}

// SendVerificationCode sends an email verification code.
func (c *Client) SendVerificationCode(ctx context.Context, to string, code string, scene string, ttl time.Duration) error {
	if !c.Configured() {
		return fmt.Errorf("mailer is not configured")
	}
	to = strings.TrimSpace(to)
	code = strings.TrimSpace(code)
	if to == "" || code == "" {
		return fmt.Errorf("email or code is empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	email := c.verificationEmailContent(code, scene, ttl)

	if strings.EqualFold(strings.TrimSpace(c.cfg.Delivery), "resend") {
		return c.sendResend(ctx, to, email.Subject, email.Text, email.HTML)
	}
	if strings.EqualFold(strings.TrimSpace(c.cfg.Delivery), "tencent_ses") {
		ttlMinutes := int(ttl.Minutes())
		if ttlMinutes <= 0 {
			ttlMinutes = 5
		}
		action := "完成邮箱验证"
		if strings.EqualFold(scene, "reset_password") {
			action = "重置密码"
		}
		return c.sendTencentSESTemplate(ctx, to, email.Subject, c.defaultTencentTemplateID(), map[string]string{
			"code":        code,
			"action":      action,
			"ttl_minutes": strconv.Itoa(ttlMinutes),
		})
	}

	message := buildMessage(c.formatFrom(), to, email.Subject, email.Text)
	return c.sendSMTP(ctx, to, []byte(message))
}

// SendPlainText sends a plain text notification email.
func (c *Client) SendPlainText(ctx context.Context, to string, subject string, textBody string) error {
	if !c.Configured() {
		return fmt.Errorf("mailer is not configured")
	}
	to = strings.TrimSpace(to)
	subject = strings.TrimSpace(subject)
	textBody = strings.TrimSpace(textBody)
	if to == "" || subject == "" || textBody == "" {
		return fmt.Errorf("email notification fields cannot be empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if strings.EqualFold(strings.TrimSpace(c.cfg.Delivery), "resend") {
		return c.sendResend(ctx, to, subject, textBody, "")
	}
	if strings.EqualFold(strings.TrimSpace(c.cfg.Delivery), "tencent_ses") {
		return c.sendTencentSES(ctx, to, subject, textBody, c.taskTencentTemplateID())
	}

	message := buildMessage(c.formatFrom(), to, subject, textBody)
	return c.sendSMTP(ctx, to, []byte(message))
}

func (c *Client) SendTaskNotification(ctx context.Context, to string, subject string, data TaskNotification) error {
	to = strings.TrimSpace(to)
	subject = strings.TrimSpace(subject)
	if to == "" || subject == "" {
		return fmt.Errorf("email notification fields cannot be empty")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	textBody := strings.TrimSpace(data.Message)
	if textBody == "" {
		textBody = strings.TrimSpace(subject)
	}
	if !c.tencentTaskConfigured() {
		return fmt.Errorf("tencent ses task notification is not configured")
	}
	return c.sendTencentSESTemplate(ctx, to, subject, c.taskTencentTemplateID(), map[string]string{
		"task_id":     strings.TrimSpace(data.TaskID),
		"status":      strings.TrimSpace(data.Status),
		"message":     textBody,
		"refund_note": strings.TrimSpace(data.RefundNote),
	})
}

func (c *Client) TaskNotificationConfigured() bool {
	return c.tencentTaskConfigured()
}

type verificationEmail struct {
	Subject string
	Text    string
	HTML    string
}

func (c *Client) verificationEmailContent(code string, scene string, ttl time.Duration) verificationEmail {
	appName := c.displayName()
	ttlMinutes := int(ttl.Minutes())
	if ttlMinutes <= 0 {
		ttlMinutes = 5
	}

	action := "完成邮箱验证"
	intro := "你正在进行邮箱验证，请使用下面的验证码完成操作。"
	if strings.EqualFold(scene, "reset_password") {
		action = "重置密码"
		intro = "你正在重置密码，请使用下面的验证码完成操作。"
	}

	subject := fmt.Sprintf("image2平台验证码：%s", code)
	text := fmt.Sprintf("%s\n\n验证码：%s\n\n用途：%s\n有效期：%d 分钟\n多次获取时，请以最新一封邮件为准。\n\n如果这不是你本人操作，可以忽略这封邮件。", intro, code, action, ttlMinutes)

	return verificationEmail{
		Subject: subject,
		Text:    text,
		HTML:    verificationEmailHTML(appName, intro, action, code, ttlMinutes),
	}
}

func verificationEmailHTML(appName string, intro string, action string, code string, ttlMinutes int) string {
	appName = html.EscapeString(appName)
	intro = html.EscapeString(intro)
	action = html.EscapeString(action)
	code = html.EscapeString(code)

	return fmt.Sprintf(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s 验证码</title>
</head>
<body style="margin:0;background:#eef3ff;color:#1f2933;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI','PingFang SC','Microsoft YaHei',sans-serif;">
  <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="background:#eef3ff;padding:32px 12px;">
    <tr>
      <td align="center">
        <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="max-width:540px;background:#ffffff;border:1px solid #d8e2ff;border-radius:24px;overflow:hidden;box-shadow:0 22px 60px rgba(30,64,175,.14);">
          <tr>
            <td style="padding:30px 32px 18px;background:linear-gradient(135deg,#f8fbff 0%%,#ffffff 58%%,#fff8e8 100%%);">
              <div style="font-size:13px;letter-spacing:.18em;text-transform:uppercase;color:#2563eb;font-weight:800;">%s · image2</div>
              <h1 style="margin:12px 0 8px;font-size:28px;line-height:1.3;color:#111827;">image2 平台验证码</h1>
              <p style="margin:0;color:#52606d;font-size:15px;line-height:1.8;">%s</p>
            </td>
          </tr>
          <tr>
            <td style="padding:0 32px 30px;">
              <div style="background:linear-gradient(135deg,#111827 0%%,#1d4ed8 100%%);border-radius:22px;padding:30px 18px;text-align:center;box-shadow:0 18px 48px rgba(29,78,216,.28);">
                <div style="font-size:14px;color:#bfdbfe;margin-bottom:12px;font-weight:700;">本次验证码</div>
                <div style="display:inline-block;background:#ffffff;color:#111827;border-radius:16px;padding:18px 22px;font-size:52px;line-height:1;letter-spacing:.24em;font-weight:900;font-family:'SFMono-Regular',Consolas,'Liberation Mono',monospace;">%s</div>
              </div>
              <table role="presentation" width="100%%" cellspacing="0" cellpadding="0" style="margin-top:18px;">
                <tr>
                  <td style="padding:13px 16px;background:#eff6ff;border-radius:14px;color:#1e3a8a;font-size:14px;line-height:1.7;border:1px solid #bfdbfe;">
                    用途：%s<br>
                    有效期：%d 分钟<br>
                    多次获取时，请以最新一封邮件为准。
                  </td>
                </tr>
              </table>
              <p style="margin:18px 0 0;color:#697586;font-size:13px;line-height:1.8;">如果这不是你本人操作，可以忽略这封邮件。</p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, appName, appName, intro, code, action, ttlMinutes)
}

func (c *Client) sendResend(ctx context.Context, to string, subject string, textBody string, htmlBody string) error {
	apiURL := strings.TrimSpace(c.cfg.ResendURL)
	if apiURL == "" {
		apiURL = "https://api.resend.com/emails"
	}

	payload := map[string]any{
		"from":    c.formatFrom(),
		"to":      []string{strings.TrimSpace(to)},
		"subject": subject,
		"text":    textBody,
		"html":    htmlBody,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("resend marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("resend create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.resendKey())
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: c.cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("resend send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("resend send failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}

func (c *Client) sendTencentSES(ctx context.Context, to string, subject string, textBody string, templateID int64) error {
	return c.sendTencentSESTemplate(ctx, to, subject, templateID, map[string]string{
		"content": textBody,
	})
}

func (c *Client) sendTencentSESTemplate(ctx context.Context, to string, subject string, templateID int64, templateVars map[string]string) error {
	if templateID <= 0 {
		return fmt.Errorf("tencent ses template id is not configured")
	}
	endpoint := strings.TrimSpace(c.cfg.TencentEndpoint)
	if endpoint == "" {
		endpoint = "ses.tencentcloudapi.com"
	}
	region := strings.TrimSpace(c.cfg.TencentRegion)
	if region == "" {
		region = "ap-guangzhou"
	}

	templateData, err := json.Marshal(templateVars)
	if err != nil {
		return fmt.Errorf("tencent ses marshal template data: %w", err)
	}

	payload := map[string]any{
		"FromEmailAddress": c.tencentFormatFrom(),
		"Destination":      []string{strings.TrimSpace(to)},
		"Subject":          subject,
		"Template": map[string]any{
			"TemplateID":   templateID,
			"TemplateData": string(templateData),
		},
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("tencent ses marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("tencent ses create request: %w", err)
	}
	now := time.Now().UTC()
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", endpoint)
	req.Header.Set("X-TC-Action", "SendEmail")
	req.Header.Set("X-TC-Version", "2020-10-02")
	req.Header.Set("X-TC-Region", region)
	req.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", now.Unix()))
	if token := strings.TrimSpace(c.cfg.TencentToken); token != "" {
		req.Header.Set("X-TC-Token", token)
	}
	req.Header.Set("Authorization", c.tencentAuthorization(endpoint, now, data))

	client := &http.Client{Timeout: c.cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("tencent ses send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("tencent ses send failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var decoded struct {
		Response struct {
			Error *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(respBody, &decoded); err != nil {
		return fmt.Errorf("tencent ses decode response: %w", err)
	}
	if decoded.Response.Error != nil {
		return fmt.Errorf("tencent ses send failed: %s %s", decoded.Response.Error.Code, decoded.Response.Error.Message)
	}
	return nil
}

func buildTaskNotificationText(data TaskNotification) string {
	var b strings.Builder
	taskID := strings.TrimSpace(data.TaskID)
	status := strings.TrimSpace(data.Status)
	message := strings.TrimSpace(data.Message)
	refundNote := strings.TrimSpace(data.RefundNote)

	b.WriteString("您好，这是一封来自 image2 平台的生图任务通知邮件，用于告知您在平台提交的图片生成任务处理结果。")
	if taskID != "" {
		fmt.Fprintf(&b, "\n任务编号：%s", taskID)
	}
	if status != "" {
		fmt.Fprintf(&b, "\n当前状态：%s", status)
	}
	if message != "" {
		fmt.Fprintf(&b, "\n任务说明：%s", message)
	}
	if refundNote != "" {
		fmt.Fprintf(&b, "\n%s", refundNote)
	}
	b.WriteString("\n如非本人操作，请登录平台检查账号安全。")
	return b.String()
}

func (c *Client) tencentAuthorization(endpoint string, now time.Time, payload []byte) string {
	const service = "ses"
	date := now.Format("2006-01-02")
	credentialScope := date + "/" + service + "/tc3_request"
	payloadHash := sha256Hex(payload)
	canonicalRequest := strings.Join([]string{
		http.MethodPost,
		"/",
		"",
		"content-type:application/json; charset=utf-8\nhost:" + endpoint + "\n",
		"content-type;host",
		payloadHash,
	}, "\n")
	stringToSign := strings.Join([]string{
		"TC3-HMAC-SHA256",
		fmt.Sprintf("%d", now.Unix()),
		credentialScope,
		sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	secretDate := hmacSHA256([]byte("TC3"+strings.TrimSpace(c.cfg.TencentSecretKey)), date)
	secretService := hmacSHA256(secretDate, service)
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))
	return fmt.Sprintf("TC3-HMAC-SHA256 Credential=%s/%s, SignedHeaders=content-type;host, Signature=%s", strings.TrimSpace(c.cfg.TencentSecretID), credentialScope, signature)
}

func (c *Client) sendSMTP(ctx context.Context, to string, message []byte) error {
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

func (c *Client) resendKey() string {
	if key := strings.TrimSpace(c.cfg.ResendKey); key != "" {
		return key
	}
	// Backward-compatible with the previous SMTP_PASSWORD based Resend setup.
	return strings.TrimSpace(c.cfg.Password)
}

func (c *Client) tencentFrom() string {
	if from := strings.TrimSpace(c.cfg.TencentFrom); from != "" {
		return from
	}
	return strings.TrimSpace(c.cfg.From)
}

func (c *Client) tencentTaskConfigured() bool {
	if c == nil {
		return false
	}
	return strings.TrimSpace(c.cfg.TencentSecretID) != "" &&
		strings.TrimSpace(c.cfg.TencentSecretKey) != "" &&
		strings.TrimSpace(c.tencentFrom()) != "" &&
		c.cfg.TencentTaskTemplateID > 0
}

func (c *Client) defaultTencentTemplateID() int64 {
	return c.cfg.TencentTemplateID
}

func (c *Client) taskTencentTemplateID() int64 {
	return c.cfg.TencentTaskTemplateID
}

func (c *Client) tencentFormatFrom() string {
	from := c.tencentFrom()
	name := strings.TrimSpace(c.cfg.TencentFromName)
	if name == "" {
		name = c.displayName()
	}
	if name == "" {
		return from
	}
	return fmt.Sprintf("%s <%s>", name, from)
}

func (c *Client) formatFrom() string {
	from := strings.TrimSpace(c.cfg.From)
	name := c.displayName()
	if name == "" {
		return from
	}
	return fmt.Sprintf("%s <%s>", mime.QEncoding.Encode("utf-8", name), from)
}

func (c *Client) displayName() string {
	name := strings.TrimSpace(c.cfg.FromName)
	if name == "" {
		name = strings.TrimSpace(c.cfg.AppName)
	}
	if name == "" {
		name = "jh612"
	}
	return name
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

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hmacSHA256(key []byte, data string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(data))
	return mac.Sum(nil)
}
