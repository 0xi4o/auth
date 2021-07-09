package api

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/netlify/gotrue/api/sms_provider"
	"github.com/netlify/gotrue/crypto"
	"github.com/netlify/gotrue/models"
	"github.com/netlify/gotrue/storage"
	"github.com/pkg/errors"
)

const e164Format = `^[1-9]\d{1,14}$`

// validateE165Format checks if phone number follows the E.164 format
func (a *API) validateE164Format(phone string) bool {
	// match should never fail as long as regexp is valid
	matched, _ := regexp.Match(e164Format, []byte(phone))
	return matched
}

// formatPhoneNumber removes "+" and whitespaces in a phone number
func (a *API) formatPhoneNumber(phone string) string {
	return strings.ReplaceAll(strings.Trim(phone, "+"), " ", "")
}

func (a *API) sendPhoneConfirmation(tx *storage.Connection, ctx context.Context, user *models.User, phone string) error {
	config := a.getConfig(ctx)

	if user.ConfirmationSentAt != nil && !user.ConfirmationSentAt.Add(config.Sms.MaxFrequency).Before(time.Now()) {
		return MaxFrequencyLimitError
	}

	now := time.Now()
	oldToken := user.ConfirmationToken
	otp, err := crypto.GenerateOtp(config.Sms.OtpLength)
	if err != nil {
		return internalServerError("error generating otp").WithInternalError(err)
	}
	user.ConfirmationToken = otp

	smsProvider, err := sms_provider.GetSmsProvider(*config)
	if err != nil {
		return err
	}

	if serr := smsProvider.SendSms(phone, user.ConfirmationToken); serr != nil {
		user.ConfirmationToken = oldToken
		return serr
	}

	user.ConfirmationSentAt = &now

	return errors.Wrap(tx.UpdateOnly(user, "confirmation_token", "confirmation_sent_at"), "Database error updating user for confirmation")
}
