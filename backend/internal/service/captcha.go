package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math/big"
	"strings"
	"time"

	"image2/backend/internal/cache"
	"image2/backend/internal/dto"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	captchaLength = 4
	captchaTTL    = 5 * time.Minute
)

var captchaAlphabet = []rune("23456789ABCDEFGHJKLMNPQRSTUVWXYZ")

func (s *Service) captchaPolicy() cache.CaptchaPolicy {
	return cache.CaptchaPolicy{
		TTL:         captchaTTL,
		MaxAttempts: 5,
	}
}

func (s *Service) GenerateCaptcha(ctx context.Context) (dto.CaptchaResponse, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.ensureRegisterEmailDailyQuota(ctx); err != nil {
		return dto.CaptchaResponse{}, err
	}

	id, err := randomCaptchaToken(24)
	if err != nil {
		return dto.CaptchaResponse{}, err
	}
	code, err := randomCaptchaCode(captchaLength)
	if err != nil {
		return dto.CaptchaResponse{}, err
	}
	if err := s.captchas.Save(ctx, id, code, s.captchaPolicy()); err != nil {
		return dto.CaptchaResponse{}, err
	}

	return dto.CaptchaResponse{
		CaptchaID: id,
		Image:     "data:image/png;base64," + base64.StdEncoding.EncodeToString(captchaPNG(code)),
		ExpiresIn: int(captchaTTL / time.Second),
	}, nil
}

func (s *Service) verifyCaptcha(ctx context.Context, id string, code string) error {
	ok, err := s.captchas.Verify(ctx, id, code, s.captchaPolicy())
	if err != nil {
		if errors.Is(err, cache.ErrCaptchaAttemptsExceeded) {
			return ErrCaptchaAttemptsExceeded
		}
		return ErrInvalidCaptcha
	}
	if !ok {
		return ErrInvalidCaptcha
	}
	return nil
}

func (s *Service) hasCaptchaPass(ctx context.Context, email string, scene string) bool {
	key := captchaPassKey(email, scene)
	if key == "" {
		return false
	}
	if s.redis != nil {
		value, err := s.redis.Get(ctx, key).Result()
		return err == nil && value == "1"
	}

	now := time.Now()
	s.captchaPassMu.Lock()
	defer s.captchaPassMu.Unlock()
	expiresAt, ok := s.captchaPasses[key]
	if !ok || now.After(expiresAt) {
		delete(s.captchaPasses, key)
		return false
	}
	return true
}

func (s *Service) rememberCaptchaPass(ctx context.Context, email string, scene string) error {
	key := captchaPassKey(email, scene)
	if key == "" {
		return nil
	}
	ttl := s.cfg.EmailCodeTTL
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	if s.redis != nil {
		return s.redis.Set(ctx, key, "1", ttl).Err()
	}

	s.captchaPassMu.Lock()
	s.captchaPasses[key] = time.Now().Add(ttl)
	s.captchaPassMu.Unlock()
	return nil
}

func captchaPassKey(email string, scene string) string {
	email = normalizeEmail(email)
	scene = strings.ToLower(strings.TrimSpace(scene))
	if email == "" || scene == "" {
		return ""
	}
	return "captcha-pass:" + scene + ":" + email
}

func randomCaptchaCode(length int) (string, error) {
	var builder strings.Builder
	builder.Grow(length)
	max := big.NewInt(int64(len(captchaAlphabet)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteRune(captchaAlphabet[n.Int64()])
	}
	return builder.String(), nil
}

func randomCaptchaToken(length int) (string, error) {
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	var builder strings.Builder
	builder.Grow(length)
	max := big.NewInt(int64(len(alphabet)))
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		builder.WriteByte(alphabet[n.Int64()])
	}
	return builder.String(), nil
}

func captchaPNG(code string) []byte {
	const width = 150
	const height = 48
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(238 + x/16),
				G: uint8(244 + y/24),
				B: 255,
				A: 255,
			})
		}
	}

	drawCaptchaLine(img, 10, 41, 140, 39, color.RGBA{147, 197, 253, 55})
	drawCaptchaDots(img, 8)
	drawScaledCaptchaText(img, code)

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

func drawScaledCaptchaText(dst *image.RGBA, code string) {
	src := image.NewRGBA(image.Rect(0, 0, 66, 18))
	draw.Draw(src, src.Bounds(), image.Transparent, image.Point{}, draw.Src)

	d := &font.Drawer{
		Dst:  src,
		Src:  image.NewUniform(color.RGBA{15, 23, 42, 255}),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(3, 14),
	}
	d.DrawString(strings.Join(strings.Split(code, ""), " "))

	scale := 2
	offsetX := 11
	offsetY := 6
	for y := src.Bounds().Min.Y; y < src.Bounds().Max.Y; y++ {
		for x := src.Bounds().Min.X; x < src.Bounds().Max.X; x++ {
			_, _, _, a := src.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					tx := offsetX + x*scale + dx
					ty := offsetY + y*scale + dy
					if image.Pt(tx, ty).In(dst.Bounds()) {
						dst.Set(tx, ty, color.RGBA{15, 23, 42, 255})
					}
				}
			}
		}
	}
}

func drawCaptchaLine(img *image.RGBA, x1 int, y1 int, x2 int, y2 int, c color.RGBA) {
	dx := x2 - x1
	dy := y2 - y1
	steps := dx
	if absInt(dy) > absInt(dx) {
		steps = absInt(dy)
	}
	if steps <= 0 {
		return
	}
	for i := 0; i <= steps; i++ {
		x := x1 + dx*i/steps
		y := y1 + dy*i/steps
		for oy := -1; oy <= 1; oy++ {
			for ox := -1; ox <= 1; ox++ {
				p := image.Pt(x+ox, y+oy)
				if p.In(img.Bounds()) {
					img.SetRGBA(p.X, p.Y, c)
				}
			}
		}
	}
}

func drawCaptchaDots(img *image.RGBA, count int) {
	for i := 0; i < count; i++ {
		x := (i*37 + 19) % img.Bounds().Dx()
		y := (i*23 + 11) % img.Bounds().Dy()
		img.SetRGBA(x, y, color.RGBA{147, 197, 253, 80})
	}
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
