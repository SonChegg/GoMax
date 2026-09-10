package auth

import "context"

// authenticateWithRegistration runs the new-account registration retry
// loop used by the SMS flow: on a rejected attempt (e.g. MAX's
// "only letters allowed" name validation), it asks the provider again and
// retries ConfirmRegistration with the same registration token instead of
// requiring a brand-new SMS code.
func authenticateWithRegistration(ctx context.Context, deps Deps, provider RegistrationProvider, token string) (string, error) {
	var lastErr error
	for {
		cfg, err := provider.GetRegistration(ctx, lastErr)
		if err != nil {
			return "", err
		}

		resp, err := deps.Auth.ConfirmRegistration(ctx, cfg.FirstName, cfg.LastName, token)
		if err != nil {
			lastErr = err
			continue
		}
		return resp.Token, nil
	}
}
